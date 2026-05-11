# Decisions Log

> **Purpose:** Append-only record of architectural and implementation choices. Every non-trivial decision goes here with rationale, so future you (and interview reviewers) can defend the design.
>
> **Format:** Each entry has Context, Decision, Why, Alternatives, Impact. Keep entries to ~5-10 lines.
>
> **Rule:** Never delete or edit a past entry. If you change your mind, add a *new* entry that supersedes it (and reference the old one).

---

## 2026-05-11 — Tech stack: Go + Postgres + Mongo + Redis + TimescaleDB + Next.js

**Context:** Need to pick stack for high-throughput IMS in a 7-day window.

**Decision:** Go for backend, Gin for HTTP, gRPC for streaming, Postgres for transactional, MongoDB for audit log, Redis for cache + debounce, TimescaleDB (as Postgres extension) for timeseries, Next.js 14 for frontend, Docker Compose for orchestration.

**Why:** Goroutines + channels map cleanly to bounded-channel backpressure. Polyglot persistence directly satisfies the assignment's "four sinks" requirement. TimescaleDB as a Postgres extension saves a container. Next.js gives server-side polling for the live feed without effort.

**Alternatives considered:**
- Rust + Axum — better concurrency story, but slower 7-day dev velocity for me.
- Node.js + Fastify — easier frontend integration, but goroutine model is a better fit for the assignment's "concurrency & scaling" rubric.
- Single Postgres for everything (with JSONB) — would work but loses the "right tool for the job" story; rubric grades data-handling separately.

**Impact:** Locks in pgx, go-redis, mongo-go-driver, and `golang-migrate` as dependencies.

---

## 2026-05-11 — Debounce via Redis Lua, not in-memory `sync.Map`

**Context:** Need atomic check-then-act for "is there an active window for this component_id?"

**Decision:** Use a Redis Lua script that performs GET/INCR/SET-with-EX atomically. Keys are `debounce:{component_id}:work_item` and `debounce:{component_id}:count`.

**Why:** Lua scripts run single-threaded server-side in Redis, eliminating the check-then-act race condition without distributed locks. Works correctly across multiple ingestion-worker replicas, which `sync.Map` would not.

**Alternatives considered:**
- In-memory `sync.Map` keyed by component_id — faster (~50ns vs ~200μs per lookup) but only works within one process. Two ingestion replicas would produce duplicate work items.
- Distributed lock per component_id — adds latency, risk of deadlock, more complex error handling.
- Single-threaded debouncer goroutine talked to via channel — works within one process, doesn't scale to replicas.

**Impact:** Adds one network round-trip to Redis per signal (~1ms). Trade-off accepted for distributed-correctness. Failure mode: if Redis is unreachable, fall through to "always CREATE" — noisier but no signal loss.

---

## 2026-05-11 — Backpressure via bounded channel + non-blocking send, return 503 when full

**Context:** Need to handle 10K signals/sec without crashing when persistence is slow.

**Decision:** A bounded Go channel of capacity 50,000 between HTTP/gRPC handlers and worker pool. Handlers use `select` with `default` case for non-blocking send. Full channel → return 503 (HTTP) or `ResourceExhausted` (gRPC) with `Retry-After: 100ms`.

**Why:** Channel capacity decouples ingestion latency from persistence latency. Caller is never blocked; backpressure propagates to the client which retries with backoff. Capacity sized for ~5s of nominal load (10K/sec × 5s = 50K).

**Alternatives considered:**
- Unbounded channel + worker pool — would OOM under sustained burst.
- Kafka or NATS as buffer — adds a container, a driver, a deployment dependency. The in-process channel meets throughput requirements at this scale.
- Drop on overload silently — violates FR-1.5 (caller must be acknowledged).

**Impact:** Channel size becomes a tunable. Documented in README backpressure section.

---

## 2026-05-11 — SERIALIZABLE isolation for Work Item state transitions

**Context:** Two concurrent requests to advance the same work item could race.

**Decision:** All state transitions wrap `SELECT FOR UPDATE` + state-pattern check + UPDATE + INSERT into a single Postgres transaction at SERIALIZABLE isolation. Loser of concurrent transitions gets 409 Conflict.

**Why:** Transitions are human-driven and low-frequency, so the perf cost of SERIALIZABLE is negligible. The audit table (`state_transitions`) needs phantom-read protection so the timeline stays consistent.

**Alternatives considered:**
- READ COMMITTED with optimistic lock (version column) — viable but more app-side logic, more failure paths.
- Application-level mutex — doesn't survive multi-replica deployment.

**Impact:** Need `SET TRANSACTION ISOLATION LEVEL SERIALIZABLE` in the workflow package's repository methods.

---

## 2026-05-11 — RCA validation lives only in `ResolvedState.CanTransitionTo`

**Context:** "Cannot CLOSE without RCA" rule could be enforced in many places (handler, service, repo, DB constraint).

**Decision:** Single enforcement point: `ResolvedState.CanTransitionTo(ClosedState, ctx)` returns `ErrMissingRCA` or `ErrIncompleteRCA`. Handler maps these to 422 with field details.

**Why:** State pattern is exactly the right place. Reviewer can find the rule in 5 seconds. Adding a new way to close (e.g. CLI) automatically inherits the rule. DB-level CHECK constraint would be redundant defense in depth, but the state-pattern check happens first and produces better errors.

**Alternatives considered:**
- Postgres CHECK constraint that requires `rca_id IS NOT NULL` when status is CLOSED — works as defense in depth, may add later if time.
- Validate in HTTP handler — distributes the rule across protocols (HTTP + gRPC), bad.

**Impact:** Unit test for this rule is the most-emphasized test in the project (rubric calls it out explicitly).

---

## 2026-05-11 — Persistence fan-out is NOT transactional across stores

**Context:** Per signal, we write to Mongo (raw), Postgres (work item), Redis (dashboard cache), TimescaleDB (metric). Should these four writes be a distributed transaction?

**Decision:** No. Each is independent. Postgres write is treated as the source of truth; others are derivatives. Each individual write has retry-with-backoff (3 attempts, exponential); final failure dead-letters to Mongo.

**Why:** Distributed transactions across heterogeneous stores require 2PC, sagas, or compensating actions — vastly more complex than the requirement justifies. Eventual consistency is acceptable: if Redis is briefly out of sync, dashboard is briefly stale; if Mongo lags, audit log catches up.

**Alternatives considered:**
- Transactional outbox pattern — clean but adds infrastructure (an outbox table + relay process). Worth it in production, not for a 7-day demo.
- Saga with compensating actions — over-engineered for the failure modes we care about.

**Impact:** The README's "consistency model" section will explicitly call this out.

---

## 2026-05-11 — Dual HTTP + gRPC ingestion sharing the same in-process channel

**Context:** Need to choose ingestion protocol(s).

**Decision:** Both HTTP (Gin) and gRPC server-streaming. Both handlers push to the same bounded `chan Signal`. One pipeline, two protocols.

**Why:** HTTP is necessary for the failure simulator script, ad-hoc curl debugging, and the dashboard's polling. gRPC streaming is the more realistic high-volume internal-source ingestion path and demonstrates a second protocol on the resume. Sharing the channel means no protocol-specific divergence downstream.

**Alternatives considered:**
- HTTP only — simpler, but misses a chance to demonstrate gRPC.
- gRPC only — harder to manually test, no curl, frontend would need grpc-web.

**Impact:** Run two listeners (ports 8080 HTTP, 9090 gRPC) in `cmd/ims/main.go`.

---

## 2026-05-11 — Worker pool sized at `runtime.NumCPU() * 2`

**Context:** How many goroutines should consume from the signal channel?

**Decision:** `runtime.NumCPU() * 2`. Configurable via env var.

**Why:** Workers spend most of their time on I/O (Redis Lua, Postgres write, Mongo write). Oversubscribing CPU cores by 2× is standard heuristic for I/O-bound work pools.

**Alternatives considered:**
- Equal to `NumCPU()` — under-utilizes CPU during I/O waits.
- Fixed (e.g. 100) — doesn't adapt to machine. Bad on small dev boxes, wasteful on big servers.
- Dynamic auto-scaling worker pool — overkill for v1.

**Impact:** On an 8-core box → 16 workers. Verified empirically by Phase 2 load test.

---

## 2026-05-11 — Phase 1: pinned image tags `timescale/timescaledb:2.17.2-pg16`, `mongo:7.0.14`, `redis:7.4.1-alpine`

**Context:** R5 (00-master-prd §10.1) requires reproducible bring-up on a reviewer's machine. Floating tags (`postgres:16`, `latest`) break that.

**Decision:** Pin every image in `docker/compose.yaml` to a specific minor (and where available, patch) version: `timescale/timescaledb:2.17.2-pg16` for Postgres+Timescale, `mongo:7.0.14`, `redis:7.4.1-alpine`.

**Why:** A reviewer cloning the repo six months from now gets bit-identical behaviour, not whatever the registry has redirected `:7` to. Postgres 16 line matches CLAUDE.md; Mongo 7 and Redis 7 match the stack table. Versions chosen are the latest patches available at scaffold time.

**Alternatives considered:**
- Floating major tags (`postgres:16`, `redis:7`) — common, but breaks reproducibility on a reviewer's machine months later.
- Digest pinning (`@sha256:...`) — bulletproof but ugly and a pain to update; overkill for a 7-day demo.

**Impact:** Anyone bumping versions must do it deliberately and log the upgrade here. Future Phase 7 dry-run on a fresh clone will confirm reproducibility.

---

## 2026-05-11 — Phase 1: TimescaleDB co-located in the Postgres container, not a separate service

**Context:** CLAUDE.md and the user prompt phrase "all 4 databases healthy." Architecturally that's misleading — TimescaleDB is a Postgres *extension*, not a separate engine.

**Decision:** One container (`timescale/timescaledb:2.17.2-pg16`) serves both Postgres and TimescaleDB. The extension is enabled by `docker/postgres/init.sql` on first boot. Backend code will use one pgx pool for both transactional tables and hypertables.

**Why:** This is the entire reason 01-architecture §3.2 picked Timescale-the-extension over Prometheus — "one less container and one less driver." Splitting into two containers would contradict the documented rationale and add an unnecessary moving part.

**Alternatives considered:**
- Run a second `postgres:16` container labelled "timescale" — would technically give "4 databases" in `docker compose ps` but is architecturally wrong and operationally wasteful.

**Impact:** `docker compose ps` shows 3 services, not 4. The README explicitly calls this out. Healthcheck on the single Postgres container covers both logical stores.

---

## 2026-05-11 — Phase 1: pnpm via corepack, with workspace-level `allowBuilds`

**Context:** CLAUDE.md prescribes `pnpm dev` but pnpm wasn't installed; pnpm 11 also blocks any dep that has post-install scripts unless explicitly allowed.

**Decision:** Enable pnpm via `corepack enable && corepack prepare pnpm@latest --activate`. Allowlist `unrs-resolver` (a transitive native-binding dep of `eslint-config-next`) in `frontend/pnpm-workspace.yaml`'s `allowBuilds` section.

**Why:** corepack ships with Node 20+, so no separate install. pnpm 11's allowlist is a security feature (defense against post-install supply-chain attacks); `unrs-resolver` is a well-known, widely-used native binding for ESLint's resolver, safe to allow.

**Alternatives considered:**
- Switch the project to npm — would diverge from CLAUDE.md and lose pnpm's lockfile determinism.
- Manually run `pnpm approve-builds` per-machine — not reproducible; would block CI.

**Impact:** Fresh clones must run `corepack enable` once. README documents this in the prereq line.

---

## 2026-05-11 — Phase 2: queue capacity 50,000, worker count `NumCPU()*2`

**Context:** The bounded channel between handlers and workers needs two numbers — its depth and the size of the consumer pool. Both are tunables, both affect the load-test outcome.

**Decision:** Default queue capacity 50,000 (5 seconds of nominal at 10K/sec). Default worker count `runtime.NumCPU() * 2` (on this 10-core M-series box = 20 workers). Both are env-overridable (`IMS_QUEUE_CAPACITY`, `IMS_WORKER_COUNT`).

**Why:** Capacity follows 01-arch §4.2 directly — enough buffer to absorb a 5-second persistence stall without immediate 503s, small enough to keep memory bounded (~20MB for 50K Signal structs). `NumCPU()*2` is the standard I/O-bound oversubscription heuristic (01-arch §4.3); workers spend most of their time waiting on Redis/Postgres/Mongo, so packing more goroutines per core is correct.

**Alternatives considered:**
- Unbounded queue — would OOM under sustained burst, violates the entire FR-1.5 / NFR-2.1 backpressure story.
- `NumCPU()` workers — under-utilizes during I/O waits; verified empirically in Phase 2 load test that NumCPU()*2 handled 10K/sec with workers always idle (queue depth pinned at 0).
- Fixed worker count (e.g. 100) — doesn't adapt to host; bad on small dev boxes.

**Impact:** Verified Phase 2: 600K requests over 60s @ 10K/sec, 100% success, p99 = 1.89 ms, 0 dropped, queue depth never exceeded a handful (workers always outpaced producers under noop processing). Real numbers will shift in Phase 3 when the processor does Redis Lua + Postgres + Mongo writes — re-measure.

---

## 2026-05-11 — Phase 2: load test bumps `IMS_RATE_LIMIT_RPS` to 20,000

**Context:** Default per-source rate limit is 1000 req/sec (FR-1.6). Vegeta runs from a single host, so every test request hits the same source IP (127.0.0.1) and would 429 immediately.

**Decision:** `scripts/load-test.sh` boots (or expects the caller to boot) the backend with `IMS_RATE_LIMIT_RPS=20000 IMS_RATE_LIMIT_BURST=40000`. The limiter is still in the request path — we just configure it so a single-host test isn't bottlenecked by it.

**Why:** The rate limiter's job is to protect the system from one chatty source (FR-1.6 says "default 1000/sec per source"). For a single-host benchmark, treating localhost as a "fleet" reflects what the production constraint actually does: rate-limit per source IP, not globally. Globally, the system is sized for 10K/sec aggregate (NFR-1.1).

**Alternatives considered:**
- Spoof X-Forwarded-For in vegeta to rotate fake IPs — overcomplicates the test, requires trusting proxies in Gin config.
- Run vegeta from multiple boxes — out of scope for a 7-day demo.
- Disable the rate limiter entirely during load test — would not exercise the limiter's overhead at all (~50ns/call). Want it in the loop so the measured p99 includes it.

**Impact:** README and load-test.sh both document this. Anyone running the script needs to know about the env override.

---

## 2026-05-11 — Phase 2: shutdown order is HTTP-then-pipeline, not parallel

**Context:** On SIGINT we need to drain in-flight HTTP requests AND in-flight queue items. If we close the pipeline first, in-flight handlers panic on a send to a closed channel.

**Decision:** Sequential shutdown in `cmd/ims/main.go`: (1) `srv.Shutdown(ctx)` blocks until HTTP handlers return; (2) `pipe.Stop()` then closes the input channel and drains workers; (3) wait on `pipe.Done()`. Total budget bounded by `IMS_SHUTDOWN_TIMEOUT` (default 30s, matches NFR-2.4).

**Why:** It's the only ordering that's race-free without adding a separate "draining" flag readable by the handler. Once `srv.Shutdown` returns, no goroutine can be inside `Submit`, so closing the channel is safe.

**Alternatives considered:**
- Atomic "draining" flag checked by Submit before send — works but adds a load per request on the hot path.
- Cancel root ctx and let everything die — handlers leak panics on the closed channel; bad UX.

**Impact:** Verified by SIGINT during the load test — server printed `[metrics-final]` summary, drained cleanly, returned exit 0.

---

## 2026-05-11 — Phase 2: rate-limiter `lastSeen` uses `atomic.Int64` (UnixNano), not a mutex

**Context:** The hot path needs to update each bucket's "last touched" timestamp on every request so the sweeper can evict idle buckets. Doing this under the map's RWMutex would force the hot path to take the write lock on every hit.

**Decision:** Store `lastSeenNano` as `atomic.Int64`. Hot path does `b.lastSeenNano.Store(now.UnixNano())` — lock-free. Sweeper reads with `.Load()` under the map's write lock (which it must hold anyway to delete entries).

**Why:** Caught by `-race` in `TestRateLimiter_ConcurrentBucketCreation`. The original draft had a plain `time.Time` field updated outside the mutex on the assumption that "stale-by-one-tick is fine" — true semantically, but a data race on a non-atomic field is undefined behaviour, not "slightly stale." Atomic Int64 is the cheapest fix that keeps the lock-free hot path.

**Alternatives considered:**
- Take the write lock on every request — kills concurrency on the hot path; rate-limiter becomes a global contention point.
- Update lastSeen inside the sweeper only — works but means a bucket's "last seen" is the last sweep tick, not its last request, so eviction is less precise.

**Impact:** `internal/ingest/ratelimit.go` is now race-clean. Worth flagging in interviews — small example of where "obviously benign" data races are not actually benign.

---

<!--
TEMPLATE for new entries — copy and fill in:

## YYYY-MM-DD — short title

**Context:** what problem prompted the decision

**Decision:** what we did

**Why:** rationale

**Alternatives considered:** what else we looked at, why rejected

**Impact:** what this changes downstream
-->
