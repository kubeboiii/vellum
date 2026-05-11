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

## 2026-05-11 — Phase 3: migrations via golang-migrate CLI (not auto-run on startup)

**Context:** SQL migrations have to run before the app can write — but where? Common patterns: (a) ship them as files run by the migrate CLI as an ops step, (b) embed migrate in the app and run on boot, (c) put everything in `init.sql`.

**Decision:** Option (a). `backend/migrations/*.sql` is run by the `migrate` CLI (installed via Homebrew). `init.sql` only enables the timescaledb extension (which needs superuser). The app's pgx pool connects with a least-privileged user that can't run DDL.

**Why:** Separates app-runtime credentials from schema-change credentials, which is the production-correct pattern. The app fails fast at startup if a migration is missing rather than silently re-running migrations on every restart. Easier rollback story (`migrate down`).

**Alternatives considered:**
- Embed migrate as a Go library and call `Migrate.Up()` at startup — convenient for dev but the app then needs DDL permissions, and "did this restart succeed" gets murky.
- Put all DDL in `docker/postgres/init.sql` — works but no versioning, no rollback, no idempotency tracking once the dev volume sticks.

**Impact:** README quick-start adds `migrate -path backend/migrations -database "$DATABASE_URL" up` between `docker compose up` and `go run ./cmd/ims`. Anyone joining the project needs `brew install golang-migrate`.

---

## 2026-05-11 — Phase 3: switched to mongo-driver v2 (not v1)

**Context:** When pulling `go.mongodb.org/mongo-driver@latest`, Go emitted a deprecation warning: the v1 line is "deprecated, use go.mongodb.org/mongo-driver/v2".

**Decision:** Use v2 (`go.mongodb.org/mongo-driver/v2`).

**Why:** v2 is the supported branch going forward; new features and fixes land there. The API shape is almost identical (same `mongo.Connect`, `Collection.InsertOne`, etc.), so the migration cost is zero for new code. v1 still works but writing greenfield code against a deprecated module would be a bad look in interviews.

**Alternatives considered:**
- Stay on v1 — works but accumulates technical debt from day one.

**Impact:** All Mongo imports use `go.mongodb.org/mongo-driver/v2/{mongo,bson,...}`. `options.Client().ApplyURI(...)` is the v2 form for connection options.

---

## 2026-05-11 — Phase 3: `*redis.Script.Run` instead of raw EVALSHA for the debouncer

**Context:** First implementation called `client.EvalSha(ctx, sha, ...)` directly with the SHA from startup's `SCRIPT LOAD`. Problem: when Redis restarts (or FLUSHDB is invoked), the script cache is empty. Every subsequent EVALSHA returns `NOSCRIPT` and falls into our "Redis-down" fallback path — even though Redis is actually reachable. The system would permanently lose debouncing after any Redis restart until the IMS backend was also restarted.

**Decision:** Use go-redis's `*redis.Script.Run` helper. It calls EVALSHA on the cached SHA (fast path); on `NOSCRIPT`, it automatically re-uploads the script body via EVAL and retries the call.

**Why:** Production-grade resilience for one extra line of code. Confirmed empirically: kill Redis → fallback creates a work_item per signal (degraded mode); restart Redis (clean cache) → next 3 signals correctly debounce into 1 work_item. The fallback only fires when Redis is truly unreachable, not when it's just been restarted.

**Alternatives considered:**
- Manually catch `NOSCRIPT` and re-call `ScriptLoad` — works but reimplements what `*redis.Script` already does, with more surface area for bugs.
- Reload the script periodically (every 30s, say) — wasteful in the steady state; doesn't help the moment-of-failure case.
- Have a background goroutine watch `/health` and re-load on Redis recovery — extra goroutine, extra coordination, no better than the helper.

**Impact:** `internal/debounce/debounce.go` constructs `redis.NewScript(scriptBody)` once in `New` and uses `script.Run(ctx, client, keys, args...)` per call. SHA returned by startup `SCRIPT LOAD` is accepted by `New()` for symmetry (and logging) but isn't stored — `*redis.Script` recomputes it. The startup `SCRIPT LOAD` still happens because it's a useful syntax check at boot time.

---

## 2026-05-11 — Phase 3: fan-out is NOT a distributed transaction (re-affirmed in code)

**Context:** Per signal we write to Mongo, Postgres, and Timescale. Question: should we use a two-phase commit (2PC) or saga pattern to make these atomic?

**Decision:** No. Each sink writes independently. Postgres is the **source of truth** for work_items; the other two are derivatives. Each write has its own retry-with-backoff + dead-letter; one failing sink doesn't block the others. The processor calls them sequentially in one goroutine — no fan-out goroutines, no parallel writes — because at 10K signals/sec, spawning 3 goroutines per signal is 30K extra goroutines/sec.

**Why:** Cross-store distributed transactions need 2PC (not supported by Mongo + Redis + pgx in any clean way) or sagas (compensating actions — way over-engineered for a 7-day demo). Eventual consistency is acceptable per 01-architecture §6.2: "Mongo audit log is eventually consistent; Redis live feed briefly stale; Postgres state is correct."

**Alternatives considered:**
- Transactional outbox pattern (write to one DB + outbox row, separate relay process writes to others) — clean for production, way too much infrastructure for v1.
- Parallel goroutine per sink — 30K extra goroutines/sec at 10K signals/sec. Not worth the latency savings.

**Impact:** `internal/processor/processor.go` documents this as a one-paragraph comment. Each sink has its own dead-letter entry per FR-8.3, and the dead_letter Mongo collection is the operator's recovery surface (NG: not auto-replayed in v1).

---

## 2026-05-11 — Phase 3: testcontainers-go for repo integration tests

**Context:** Repository code (work_item_repo, signal_repo, debouncer) is mostly SQL/BSON/Lua strings — unit-testing those with mocks just tests that we mocked them correctly. Real tests need real backends.

**Decision:** Each repo package has a test file that spins up an ephemeral container (postgres / mongo / redis) via `testcontainers-go`, runs the production code against it, and tears down on `t.Cleanup`. The pg test also applies all migration files so the schema matches production.

**Why:** Caught a real bug: the rate-limiter atomic test (Phase 2) only triggered under -race against real concurrency. Same will be true for repo code — real DBs have real serialization quirks. Cost: ~5–15s container startup per test file (acceptable; runs in CI in parallel). Phase 6's integration test will exercise the *whole* stack end-to-end; Phase 3's per-repo tests are the finer-grained safety net.

**Alternatives considered:**
- In-memory fakes for each repo — fast but tests the fake, not the real driver behaviour.
- One big integration test in Phase 6 only — too late to catch repo-level bugs.
- Run tests against the docker-compose stack — works but you have to remember to start it; CI complications.

**Impact:** Test-only dependency on `testcontainers/testcontainers-go` + `modules/{postgres,mongodb,redis}`. The production binary is unaffected (testcontainers is in the `test` build only). `make test` would take ~30s longer than pure unit tests, which is fine.

---

## 2026-05-11 — Phase 3: `IncrementSignalCount` uses GREATEST() not naive UPDATE

**Context:** When the debouncer returns `JOINED`, we UPDATE the existing work_item's `signal_count` and `last_signal_ts`. With N workers running in parallel, signal A (timestamp T+100ms) might be PROCESSED before signal B (timestamp T+50ms) — they were both accepted by the channel, workers consume them out of order.

**Decision:** `SET last_signal_ts = GREATEST(last_signal_ts, $2)`. The new value is only written if it's actually newer than what's already there.

**Why:** Without this, the displayed "last signal at" timestamp could flicker backward as out-of-order processing happens. Tiny issue (~50ms jitter) but a frustrating "why does the UI go backward?" bug to debug. GREATEST() in Postgres is cheap and makes the column **monotonic** by construction. The signal_count is `+=1` regardless (we want to count all of them, order-independent).

**Alternatives considered:**
- Sort signals by timestamp in the worker before UPDATE — adds latency to the hot path, doesn't actually solve it (workers are independent).
- Use a window function / CTE to find the max — way more SQL for no extra correctness.

**Impact:** One inline `GREATEST` call in `pg.WorkItemRepository.IncrementSignalCount`. Documented inline. Phase 4's state transitions don't need this trick — those are SERIALIZABLE transactions, but signal-count bumps are not (deliberately, to avoid serialization contention at 10K/sec).

---

## 2026-05-11 — Phase 4: State pattern via interface, one concrete type per state

**Context:** The Work Item lifecycle (OPEN→INVESTIGATING→RESOLVED→CLOSED) needs to be encoded so that (a) the rules are easy to reason about, (b) adding a state is a one-file change, and (c) the rubric's explicit "LLD: State pattern" line is satisfied.

**Decision:** A `workflow.State` interface with three methods (`Name`, `CanTransitionTo`, `OnEnter`) and four concrete types (`OpenState`, `InvestigatingState`, `ResolvedState`, `ClosedState`). State-specific behaviour lives in the type. `ResolvedState.CanTransitionTo(ClosedState, ctx)` is the ONE place the "RCA required" rule (CLAUDE.md design rule 3) is enforced.

**Why:** Concrete types per state give us natural method receivers for state-specific behaviour. `ClosedState.OnEnter` computes MTTR; no other state needs it; the code lives with the state. A reviewer asking "where do we enforce the RCA rule?" finds one Go file with one method. Switch-based alternatives would scatter the rule across the workflow engine + every place that "decides to close" something.

**Alternatives considered:**
- Single State struct with a `name` field + lookup tables — smaller code but loses room for state-specific OnEnter side effects (MTTR).
- Pure data table (map of allowed transitions) — even smaller; can't carry OnEnter logic at all.

**Impact:** Phase 5's "ACKNOWLEDGED" or "REOPENED" state (if we add it as a bonus) is one new file. The State pattern is the largest single rubric category (LLD = 20%) so this defense matters.

---

## 2026-05-11 — Phase 4: SERIALIZABLE isolation + SELECT FOR UPDATE for state transitions

**Context:** Two concurrent `PATCH .../state` requests on the same WI must NOT both succeed. The state machine in code is consistent, but without DB-level locking the second request could read the row before the first commits and both insert state_transition audit rows.

**Decision:** The workflow engine's `Transition` method opens a SERIALIZABLE pgx tx, runs `SELECT ... FROM work_items WHERE id = $1 FOR UPDATE`, evaluates the State pattern, writes both the UPDATE and the INSERT (audit row), then commits. SERIALIZABLE protects against phantom reads on `state_transitions`; SELECT FOR UPDATE locks the work_items row so a concurrent transaction blocks until we finish.

**Why:** Concurrency contention is low (transitions are human-driven, not high-frequency), so the perf cost of SERIALIZABLE is negligible. The audit table needs phantom-read protection — at READ COMMITTED, two concurrent CLOSEs could both insert "OPEN→CLOSED" audit rows for what is logically one transition. SERIALIZABLE + row lock = exactly one wins, the other returns 409.

**Alternatives considered:**
- READ COMMITTED + optimistic version column on work_items — viable but adds app-side retry on serialization conflicts.
- Application-level mutex per work_item_id — doesn't survive multi-replica deploys.

**Impact:** Verified via `TestEngine_ConcurrentClose_ExactlyOneWins` (2 goroutines, same WI, both with valid RCAs → exactly 1 success, exactly 1 rca row, exactly 1 final transition).

---

## 2026-05-11 — Phase 4: compound POST /rca endpoint atomically closes the work item

**Context:** Submitting an RCA is logically two operations: (a) insert the rca row, (b) transition RESOLVED→CLOSED. Doing them in separate transactions opens a window where the RCA exists but the WI is still RESOLVED (or vice versa).

**Decision:** `workflow.Engine.CloseWithRCA` runs both writes (INSERT rca + UPDATE work_items + INSERT state_transition) in the SAME SERIALIZABLE transaction. The State pattern still gates the close — `ResolvedState.CanTransitionTo(ClosedState, ctx)` runs inside the tx, so the "RCA must be valid" rule fires before any rows are persisted.

**Why:** Atomicity for the user-visible contract. If a reviewer's POST /rca returns 201, both rows exist; if it returns anything else, neither exists. No "half closed" states, no orphaned RCAs.

**Alternatives considered:**
- Two endpoints (POST /rca to insert, PATCH /state to close) — what 03-api-contract draft originally suggested. Loses atomicity; the client has to coordinate.
- Saga with compensating delete — vastly over-engineered for one tx-scoped pair of writes.

**Impact:** Single endpoint, single transaction, no race window. The State pattern's enforcement is unchanged — RCA validation is still the SAME code path as a hypothetical PATCH state=CLOSED.

---

## 2026-05-11 — Phase 4: alerter dispatch is fire-and-forget in a goroutine, NOT retryable

**Context:** Alerts on Work Item creation are FYI — they tell humans something happened. If the alerter is slow or down, the worker should not pile up on it.

**Decision:** `processor.dispatchAlert(wi)` runs in a fresh goroutine with its own 5-second timeout context (FR-6.4). Errors are logged but never returned to the worker, never dead-lettered. A slow Slack webhook does not block the next signal.

**Why:** FR-6.4 is explicit: "a failing alerter cannot block ingestion or workflow." Retrying alerts via the sink-style backoff path would create exactly the failure mode the requirement prohibits. Alerts are not source of truth; if PagerDuty was down for 30 seconds, you don't want to fire 30 seconds of catch-up pages — you want to skip.

**Alternatives considered:**
- Retry+backoff like Mongo/Postgres writes — wrong semantics, violates FR-6.4.
- Push to a dedicated bounded "alerts channel" with its own worker pool — over-engineered for v1, debounced creates are rare.

**Impact:** One goroutine per CREATED work item (~10/sec at debounced steady state). At 10K signals/sec the debounce reduction ratio means we still create ≤ 100 work items/sec under load — well under "10K extra goroutines/sec" worry.

---

## 2026-05-11 — Phase 4: workflow.TxRunner adapter pattern (pg.WorkflowTxRunner)

**Context:** Go's structural interface satisfaction works for *values* but not for *return types*. `pg.WorkItemRepository.BeginTx` returns `*workItemTx` (concrete); `workflow.TxRunner.BeginTx` expects `workflow.Tx` (interface). Even though `*workItemTx` satisfies `workflow.Tx`, the method signatures don't match exactly.

**Decision:** Introduce `pg.WorkflowTxRunner` as a thin adapter: holds a `*WorkItemRepository`, its `BeginTx` delegates and returns the result typed as `workflow.Tx` (the interface). main.go does `workflow.NewEngine(pg.NewWorkflowTxRunner(workItems))`.

**Why:** The adapter is 25 lines and makes the conversion explicit. Alternatives — changing pg.BeginTx's signature to return `workflow.Tx`, or generics — would either tightly couple pg to workflow or add complexity for a one-off conversion.

**Alternatives considered:**
- Change `pg.WorkItemRepository.BeginTx` signature to return `workflow.Tx` directly — couples the persistence package to the workflow interface. Bad direction.
- Use Go generics — overkill for a single adapter.

**Impact:** One small adapter file. Phase 5's gRPC server (if it adds another workflow operation) can wire through the same `WorkflowTxRunner`.

---

## 2026-05-11 — Phase 4: API package separate from internal/ingest

**Context:** Phase 2 put HTTP handlers in `internal/ingest`. Phase 4 added 4 new endpoints (incidents list/detail + state PATCH + RCA POST) — do they go in ingest too, or get their own package?

**Decision:** New `internal/api` package. ingest stays for ingestion-only handlers (POST /v1/signals) which have a different lifecycle (rate-limited, hot path, non-blocking enqueue). api has handlers that operate on stored data via workflow.Engine and the pg repos.

**Why:** Different concerns + different dependencies. `ingest` only needs `pipeline.Submitter`; `api` needs `workflow.Engine` + `pg.WorkItemRepository` + `pg.RCARepository`. Putting them in one package would muddle the dependency graph and force ingest to import workflow (which it shouldn't).

**Alternatives considered:**
- One big `internal/api` package containing both — heavier deps for the hot-path handler.
- Per-endpoint packages — too granular.

**Impact:** `internal/api` has no test file yet — the workflow-tx integration tests in `internal/persist/pg/transition_test.go` exercise the same code paths through the engine. Phase 5 or 6 may add httptest-style handler tests.

---

## 2026-05-11 — Phase 5: buf with LOCAL plugins, not remote

**Context:** `buf generate` can pull plugins (`buf.build/protocolbuffers/go`, `buf.build/grpc/go`) over the network. Convenient but fragile — when we tried it, the remote returned "the server hosted at that remote is unavailable" mid-build.

**Decision:** Use local protoc plugins installed in `$GOPATH/bin` (`protoc-gen-go`, `protoc-gen-go-grpc`). `buf.gen.yaml` references them via `local: protoc-gen-go`. Build only requires `buf` + the two plugins; no network call during code-gen.

**Why:** Reproducibility. A 7-day demo can't depend on a vendor's CDN availability. Local plugins are one Homebrew/`go install` step per dev and then never touch the network again. The generated output is identical to the remote variant.

**Alternatives considered:**
- Remote plugins (`buf.build/...`) — convenient until they break, then you can't `buf generate` at all.
- Plain `protoc` with manual flags — more flags to remember, no lint integration.

**Impact:** README documents the install. CI (if we ever add it) caches `~/go/bin/protoc-gen-go*` rather than depending on buf.build.

---

## 2026-05-11 — Phase 5: proto laid out as `proto/ims/v1/signals.proto`

**Context:** buf's STANDARD lint rule requires `package ims.v1` to live in directory `ims/v1` relative to the proto root. We hit the warning on first generate.

**Decision:** Restructure: `backend/proto/ims/v1/signals.proto`. `go_package` is `github.com/kubeboiii/ims/proto/ims/v1;imsv1`. Generated stubs land alongside the .proto.

**Why:** Buf's "package = directory" convention is the canonical layout. Future protos (e.g. `ims.v1.WorkflowService`) get their own file under `ims/v1/`; a hypothetical `ims/v2/...` migration becomes a sibling directory.

**Alternatives considered:**
- Rename package to bare `ims` — dodges the warning but loses the version namespace; v2 evolution becomes hard.
- Suppress the lint rule — same problem in less explicit form.

**Impact:** Import path is `imsv1` in Go. Generated files are checked into git so reviewers don't need protoc installed to `go build`.

---

## 2026-05-11 — Phase 5: bidi-streaming gRPC + bytes payload (not Struct)

**Context:** FR-1.2 requires a streaming gRPC ingestion endpoint. Two protobuf choices: payload as `google.protobuf.Struct` (typed JSON tree) vs `bytes` (raw JSON blob).

**Decision:** `bytes payload = 7`. Client sends serialised JSON, server stores it as native BSON via one `json.Unmarshal` in the existing processor.

**Why:** Performance + symmetry. Struct forces a JSON-shaped *protobuf* tree which costs ~µs to re-encode on both ends and creates a mismatch with the HTTP path (which already takes JSON bytes). Bytes preserves the original blob verbatim — same code path as HTTP. The cost is "no protobuf-level validation of payload shape," which we don't want anyway (payloads are intentionally schemaless per FR-3.4).

**Alternatives considered:**
- `google.protobuf.Struct` — typed but doubles serialisation work and forks the processor path.
- Per-source-type oneof — would force every observability tool's schema into the proto, defeats the heterogeneous-payload goal.

**Impact:** gRPC and HTTP share the same `model.Signal.Payload` (`json.RawMessage`) all the way to Mongo. Adding a new observability tool that emits weird JSON requires zero proto changes.

---

## 2026-05-11 — Phase 5: gRPC server-side rate limiting is deferred

**Context:** The HTTP handler has per-source token-bucket rate limiting (Phase 2). The gRPC handler currently has none. Should it?

**Decision:** No rate limiter on the gRPC path for v1. The pipeline's bounded channel still backpressures (the gRPC handler returns `Ack_ACK_STATUS_REJECTED_QUEUE_FULL` when Submit fails), but there's no per-peer cap on accept rate.

**Why:** gRPC peers are typically long-lived internal services emitting many signals on one HTTP/2 connection — different threat model from HTTP-per-request. The natural backpressure (server controls when it reads from the stream + queue-full rejects) is adequate. A proper limiter would be a gRPC interceptor and earns its place when we actually run multiple gRPC peers, which we don't in v1.

**Alternatives considered:**
- Reuse the HTTP `RateLimiter` keyed on gRPC peer address — works, but a single peer connection ≠ a single request, so per-RPC rate doesn't map cleanly to per-source rate.
- Add a per-stream message rate — meaningful, but Phase 6+ territory.

**Impact:** The PRD's FR-1.6 is interpreted as "per-source on HTTP." Phase 6 (resilience) can revisit if a chaos test reveals a gRPC peer hammering the pipeline harder than backpressure allows.

---

## 2026-05-11 — Phase 5: dashboard polls every 2s instead of SSE/WebSocket

**Context:** FR-7.1 specifies the live feed "auto-refreshes every 2 seconds." Three options: client-side polling, Next.js ISR (`revalidate: 2`), Server-Sent Events / WebSocket.

**Decision:** Client-side polling via `useEffect` + `setInterval`. The page is `'use client'`.

**Why:** Simplest mental model that satisfies the requirement. One tab → one outbound request every 2s → trivial load. ISR would cache per-route (not per-user) and accidentally serve stale data to user N when user 1 triggered the refresh. SSE/WebSocket is on the explicit bonus list (PRD B2) and adds a stateful backend endpoint that complicates Phase 6 chaos testing.

**Alternatives considered:**
- ISR — clever but the 2s revalidate is per-route, which means all dashboard users share the same cached response. Unwanted coupling.
- WebSocket push — PRD B2 bonus. Worth doing if Day 7 has slack; not before.

**Impact:** Adding WebSocket later is a frontend-only change to `app/page.tsx` plus a `/v1/incidents/stream` endpoint on the backend. The polling client code becomes the fallback path.

---

## 2026-05-11 — Phase 5: skipped shadcn/ui, hand-rolled Tailwind components

**Context:** CLAUDE.md prescribes shadcn/ui. We tried `pnpm dlx shadcn@latest init`; the CLI hung indefinitely (~3 min) and produced no `components.json`. Killed and reconsidered.

**Decision:** Skip shadcn for v1; write a few small Tailwind components by hand (`SeverityBadge`, `StatusBadge`, inline `Field` wrapper). Total: ~60 lines of presentational code across two files.

**Why:** End-state is identical — shadcn's value is "copy ownable components into your repo"; hand-writing those same components gets us there directly. Pages render correctly, `pnpm build` is clean, no Radix dep in the bundle. We can re-introduce shadcn in Phase 7 if a particular component (combobox, dialog) gets complex.

**Alternatives considered:**
- Wait for the dlx to complete — already waited 3 min, no progress.
- Install shadcn globally via `npm i -g shadcn` then init — same dlx code path, likely same hang.
- Use Mantine / Chakra / MUI — bigger runtime dep, also not in CLAUDE.md.

**Impact:** Frontend stays at ~98 KB First Load JS (Tailwind only, no Radix). Phase 7 polish may add shadcn for the more complex pages if we add them.

---

## 2026-05-11 — Phase 5: SQLSTATE 40001 → 409 Conflict (not 500)

**Context:** Under load (gRPC streaming 10 signals at once) we saw the workflow engine's `Commit` fail with `could not serialize access due to concurrent update (SQLSTATE 40001)`. The processor's `IncrementSignalCount` UPDATE was racing with the workflow's SERIALIZABLE transaction. Default behaviour: `fmt.Errorf("pg: commit: %w", err)` propagates → API handler returns 500.

**Decision:** Match `*pgconn.PgError.Code == "40001"` in a `wrapPgError` helper that elevates it to a `pg.ErrSerializationFailure` sentinel. The API error helper maps that sentinel to **409 Conflict** with `{"error":"concurrent update detected; please retry"}`.

**Why:** 40001 is not a server bug — it's Postgres correctly preserving SERIALIZABLE semantics. The client should retry. 500 implies broken backend; 409 signals "retry the same request" exactly. Matches 01-architecture §7.2.1's stated contract: "two concurrent requests to close the same incident cannot both succeed... whoever loses retries or gets a 409 Conflict."

**Alternatives considered:**
- Auto-retry server-side — opaque to the client, harder to debug, can mask actual bugs.
- Stay at 500 — wrong semantics; clients can't tell retryable from real failures.
- Use `READ COMMITTED` to dodge 40001 entirely — would break Phase 4's audit-table phantom-read protection.

**Impact:** Frontend's PATCH/POST flow can implement a simple retry-on-409 loop. Phase 6's stress test will deliberately exercise this path.

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

## 2026-05-11 — Phase 5 Tier-1 dashboard buildout
**Context:** After landing-page polish, the dashboard itself was a thin shell. Plenty of backend data was being captured but invisible to the UI.
**Decision:** Built the entire Tier-1 list against the existing API (no backend changes, no new dependencies). Six new components for /dashboard, five for /incidents/[id], four for /incidents/closed, three new top-level routes (/postmortem, /load-test, /simulate, /flow). Plus a HealthStrip mounted under the Nav on every dashboard route. PersonaSwitcher rearranges the live feed per PRD §6 persona. Web-Audio P0 beep hooked to the unmute toggle.
**Why:** Maximum visible value per hour. Tier-2 + Tier-3 items needed schema or new endpoints; Tier-1 didn't.
**Alternatives considered:** WebSocket live feed (PRD §11 B2) and time-travel scrubber (B3) — left as future work; both Tier-3 and visible-but-not-foundational.
**Impact:** Bundle: /dashboard 110→112kb (+2kb), /incidents/[id] 3.5→13kb (+9.5kb for histograms/fingerprints/timelines), /incidents/closed 4.8→7.4kb. New routes are ~5kb each. All 11 routes prerender, all return 200 under prod server.

## 2026-05-11 — Why we kept StatCards alongside SeverityStackedBar
**Context:** The new SeverityStackedBar shows the active P0/P1/P2/P3 split as a single bar. The four existing StatCards show counts plus deltas plus sparklines.
**Decision:** Keep both. The bar answers "what's the *mix* right now" at a glance; the StatCards answer "how is each number *trending*."
**Why:** Different questions deserve different visuals. Cutting the StatCards would lose the delta + sparkline context.

## 2026-05-11 — CategoryBreakdown is lazy-loaded
**Context:** Need RCA root_cause_category breakdown for closed incidents, but /v1/incidents/closed doesn't include the RCA.
**Decision:** Render a "compute" button; on click, fire Promise.all of getIncident() over the first 50 closed items.
**Why:** Eager fetch would make the closed page issue 100 round-trips on load — unacceptable performance. Lazy is a fine UX trade for a secondary analytics widget.

## 2026-05-11 — /load-test client-side count cap
**Context:** /load-test fires real POSTs to /v1/signals; a typo could DoS the user's own laptop.
**Decision:** Hard cap count at 10,000 and rps at 5,000 client-side.
**Why:** Backend designed for 10k/s sustained — 5k/s burst is well within safe range. 10k total signals keeps any single test under 2s at max rps.

## 2026-05-11 — Web Audio P0 beep instead of MP3
**Context:** PRD wants a sound alert when a new P0 lands.
**Decision:** Synthesize a 880Hz square-wave beep via Web Audio API instead of bundling an audio asset.
**Why:** No bundle cost, no autoplay-policy surprises (the user has just clicked "unmute"), no asset to choose / license.

## 2026-05-11 — Phase 6: stress test isolates the across-window property
**Context:** TestProcess_ConcurrentSameComponent (Phase 3) already tested single-window atomicity by bumping MaxSignals = N+1 so contention couldn't cross a window boundary.
**Decision:** Added a separate TestProcess_StressManyWindows in backend/internal/debounce/stress_test.go that wires the production cap (100) and asserts: 300 concurrent goroutines → exactly 3 distinct work_items, exactly 3 CREATED actions.
**Why:** A race in the Lua window-transition code would not show up in the single-window test. Now both properties have a dedicated test.
**Impact:** Closes PRD risk R2 ("concurrency bugs hide in tests") for the cap-crossing case.

## 2026-05-11 — Phase 6: integration test against the LIVE backend
**Context:** A full lifecycle test (POST signal → debounce → state machine → RCA gate → CLOSED) needs the real four-store stack.
**Decision:** Wrote backend/internal/e2e_integration_test.go behind the `integration` build tag, pointing at the running backend on :8080. Skipped testcontainers for the four-store setup because docker-compose already provides one consistent stack — testcontainers would have duplicated the wiring.
**Why:** Build-tagged so default `go test ./...` stays fast; explicit `-tags=integration` for the slow path.
**Alternatives considered:** Spinning up testcontainers for all four DBs inside the test — would have added ~20s setup per run and duplicated docker-compose's job.
**Impact:** Headline assertion: 0.29s end-to-end including RCA-gate rejection. Passes deterministically.

## 2026-05-11 — Phase 6: simulator script complements the /simulate UI
**Context:** The /simulate page (Phase 5) is a click-and-watch demo. PRD §13 explicitly wanted a headless script too.
**Decision:** scripts/simulate-outage.go provides the same three scenarios, runnable from a clean checkout with `go run`. Computes the debounce ratio per scenario and prints an aggregate. Tolerates 503s and 5xx — they're counted, not fatal.
**Why:** Reviewers can demo the system without spinning up the frontend. The script also produces shareable terminal output for a recorded demo.
**Impact:** Cache scenario hits 100× compression; aggregate 51× (RDBMS cascade and MCP fan-out drag the harmonic mean down, by design).

## 2026-05-11 — Phase 7: Mermaid over PNG for architecture diagrams
**Context:** The PRD §13 calls for an architecture diagram. The repo had ASCII diagrams in `docs/01-architecture.md` §2 and §3.
**Decision:** Replaced the ASCII with Mermaid diagrams. GitHub renders Mermaid inline; the source is text so it diff-able in git; no image file to keep in sync. Two diagrams: system context (producers → IMS → consumers) and runtime topology (backend with all internal stages + the four data stores).
**Why:** Diagrams that live in code never go stale alongside the code. PNG/SVG exports from design tools need re-rendering and someone usually forgets.
**Alternatives considered:** Lucidchart export to PNG (better layout, harder to update); D2 (newer text-to-diagram syntax, less universally rendered).
**Impact:** A copy of the high-level pipeline diagram sits at the top of the README so the elevator pitch is visual on first scroll.

## 2026-05-11 — Phase 7: `prompts.md` as narrative, not transcript dump
**Context:** PRD §13 asks for a `prompts.md`. The literal interpretation would be a dump of every prompt Claude was given — 50+ pages of mostly noise.
**Decision:** Wrote a narrative — one section per phase — covering goal, approach, what worked, what got pushed back on, what I'd do differently. Plus a meta section on how I worked with Claude across the seven phases.
**Why:** A reviewer reads `prompts.md` to understand *process*, not to audit every keystroke. The narrative form proves understanding; the transcript dump proves only that the prompts existed.
**Impact:** ~270 lines, readable in 5 minutes, covers all 7 phases.

## 2026-05-11 — Phase 7: no recorded demo video
**Context:** PRD §13 explicitly requests a demo video. I can't reliably record video from this environment.
**Decision:** Replaced the video with a "How to demo" section in README.md showing the three commands that prove G1, G2, G3 plus a dashboard click-through guide. A reviewer can record their own video using these as the script.
**Why:** Honest about the limitation; the README section is reproducible and version-controlled. A video would be a single point of staleness.
**Impact:** Reviewer can follow the README to demonstrate all three PRD goals end-to-end in under 5 minutes.

## 2026-05-11 — Phase 7: `.env.example` audited against `os.LookupEnv` calls
**Context:** Pre-Phase-7 `.env.example` had 4 variables. The backend actually reads 18.
**Decision:** Expanded `.env.example` to cover every `envOr` / `envInt` / `envFloat` / `envDur` call in `cmd/ims/main.go`, plus the `IMS_BASE_URL` consumed by the integration test. Grouped them by subsystem with FR references.
**Why:** A reviewer who hits a config issue on day one should find the answer in `.env.example`, not by grepping the source.
**Impact:** `.env.example` is now an exhaustive, documented reference; every variable has its default value shown.
