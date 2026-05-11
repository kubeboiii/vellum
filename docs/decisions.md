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

<!--
TEMPLATE for new entries — copy and fill in:

## YYYY-MM-DD — short title

**Context:** what problem prompted the decision

**Decision:** what we did

**Why:** rationale

**Alternatives considered:** what else we looked at, why rejected

**Impact:** what this changes downstream
-->
