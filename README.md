# IMS — Mission-Critical Incident Management System

A backend-heavy distributed system that ingests failure signals at **10K/sec**,
debounces them into work items, runs them through a state-machine lifecycle
with mandatory RCA on closure, and exposes a Next.js dashboard for triage.

Built as a 7-day engineering assignment. Currently at **Phase 3 (Debounce & Persistence Fan-out)** — 10K signals/sec sustained with full polyglot persistence; debounce reduction ratio 100×.

For the full picture, read in order:

1. [`docs/00-master-prd.md`](docs/00-master-prd.md) — what and why
2. [`docs/01-architecture.md`](docs/01-architecture.md) — how it's built
3. `docs/02-data-models.md` — schemas *(Phase 2+)*
4. `docs/03-api-contract.md` — endpoints *(Phase 2+)*
5. `docs/phases/phase-N-*.md` — day-by-day prompts *(TBD)*

---

## Quick start

Requires Docker (with Compose v2), Go 1.22+, Node 20+, pnpm (via `corepack enable`), and the golang-migrate CLI (`brew install golang-migrate`).

```bash
# 1. Bring up the four data stores (Postgres+TimescaleDB, Mongo, Redis).
docker compose -f docker/compose.yaml up -d

# 2. Wait for healthchecks (~10s on a warm host).
docker compose -f docker/compose.yaml ps

# 3. Apply SQL migrations (creates work_items, state_transitions, signal_metrics hypertable).
export DATABASE_URL="postgres://ims:ims@localhost:5432/ims?sslmode=disable"
migrate -path backend/migrations -database "$DATABASE_URL" up

# 4. Run the backend on :8080.
cd backend && go run ./cmd/ims
# -> curl http://localhost:8080/health  =>  200, all deps `up` with latencies
# -> curl -X POST http://localhost:8080/v1/signals \
#         -H 'Content-Type: application/json' \
#         -d '{"component_id":"RDBMS_PRIMARY_01","component_type":"RDBMS","severity":"P0","source":"datadog","payload":{"err":"oom"}}'
#    => 202 {"signal_id":"...","status":"accepted"}
#    Signal lands in Mongo (audit), debounced via Redis Lua, work_item in Postgres, metric in Timescale.

# 4. Run the frontend (Phase 1 = placeholder page).
cd frontend && pnpm dev
# -> http://localhost:3000
```

Tear down:

```bash
docker compose -f docker/compose.yaml down       # keep volumes
docker compose -f docker/compose.yaml down -v    # nuke volumes
```

## Repo layout

See `01-architecture.md` §10. High-level:

```
backend/   Go service (cmd/ims, internal/{ingest,pipeline,debounce,workflow,...})
frontend/  Next.js 14 dashboard (App Router, Tailwind, shadcn/ui)
docker/    compose.yaml + init.sql for the Postgres+Timescale container
docs/      PRD, architecture, phase files, decisions log
scripts/   Load test, failure simulator (Phase 2+)
```

## Tech stack

| Layer | Choice | Pinned version |
|---|---|---|
| Backend | Go + Gin + gRPC | `go.mod` |
| HTTP framework | Gin | latest at scaffold |
| RDBMS | PostgreSQL + TimescaleDB | `timescale/timescaledb:2.17.2-pg16` |
| Document store | MongoDB | `mongo:7.0.14` |
| Cache | Redis | `redis:7.4.1-alpine` |
| Frontend | Next.js 14 (App Router) | `next@14.2.35` |
| Styling | Tailwind 3 + shadcn/ui | scaffold |
| Orchestration | Docker Compose | v2 |

Image tags are pinned to keep `docker compose up` reproducible on a fresh
clone (R5 in 00-master-prd §10.1). **Do not bump versions without an entry
in `docs/decisions.md`.**

## Phase 1 acceptance (Foundation)

- [x] `docker compose -f docker/compose.yaml up` brings Postgres (with the
      TimescaleDB extension loaded), MongoDB, and Redis to a `healthy` state.
- [x] `cd backend && go run ./cmd/ims` starts a Gin server on `:8080`.
- [x] `curl http://localhost:8080/health` returns `200 OK`.
- [x] `cd frontend && pnpm build` succeeds.
- [x] `cd backend && go test -race ./...` passes.
- [x] All four logical data stores (Postgres, TimescaleDB, MongoDB, Redis)
      are reachable on the published ports.

> **Note on "4 databases":** Postgres and TimescaleDB live in the same
> container (TimescaleDB is a Postgres extension — see 01-architecture §3.2
> and §12). That's a deliberate choice to reduce ops surface and is the
> standard deployment pattern.

## Phase 2 acceptance (Ingestion & Backpressure)

- [x] `POST /v1/signals` accepts a single JSON signal and returns 202
      with `{"signal_id":"...","status":"accepted"}`.
- [x] Returns 400 on validation failure, 429 on rate limit, 503 when the
      queue is full (with `Retry-After: 1` header).
- [x] Bounded `chan model.Signal` (default capacity 50,000) feeds a worker
      pool (default `runtime.NumCPU() * 2`).
- [x] Per-source token-bucket rate limiter (`golang.org/x/time/rate`),
      default 1000 req/s with burst 2000 (FR-1.6).
- [x] `/health` returns 200 with queue depth, capacity, and atomic
      counters; flips to 503 when the queue is >95% full.
- [x] Stdout metrics line every 5s: `[metrics] accepted=X/s processed=Y/s
      queue=D/C errors=E/s total_accepted=… total_dropped=…`.
- [x] Graceful shutdown on SIGINT/SIGTERM: HTTP listener stops first,
      then the pipeline drains within `IMS_SHUTDOWN_TIMEOUT` (default 30s).
- [x] **Load test:** `./scripts/load-test.sh` reports 10,000 req/s sustained
      for 60s, 100% success, p99 = 1.89 ms (target ≤ 50 ms), 0 dropped.

### Phase 2 config (env vars, with defaults)

| Var | Default | Purpose |
|---|---|---|
| `IMS_HTTP_ADDR` | `:8080` | bind address |
| `IMS_QUEUE_CAPACITY` | `50000` | bounded-channel depth (~5s of nominal at 10K/s) |
| `IMS_WORKER_COUNT` | `NumCPU()*2` | consumer goroutines |
| `IMS_RATE_LIMIT_RPS` | `1000` | per-source token refill rate (FR-1.6) |
| `IMS_RATE_LIMIT_BURST` | `2000` | per-source burst tolerance |
| `IMS_METRICS_INTERVAL` | `5s` | stdout metrics cadence (FR-8.2) |
| `IMS_SHUTDOWN_TIMEOUT` | `30s` | drain deadline (NFR-2.4) |

### Running the load test yourself

```bash
# Terminal 1 — boot the backend with rate limit lifted for single-host benchmark
cd backend && IMS_RATE_LIMIT_RPS=20000 IMS_RATE_LIMIT_BURST=40000 go run ./cmd/ims

# Terminal 2 — run vegeta
./scripts/load-test.sh   # RATE=10000 DURATION=60s
```

The script writes vegeta artifacts to `.loadtest/` (gitignored).

## Phase 3 acceptance (Debounce & Persistence Fan-out)

- [x] SQL migrations create `work_items`, `state_transitions`, and the
      TimescaleDB `signal_metrics` hypertable. `down` then `up` is idempotent.
- [x] Per signal, the processor:
      (1) atomically debounces via the Redis Lua script
      (`backend/internal/debounce/script.lua`, loaded with `SCRIPT LOAD`),
      (2) writes the raw signal to Mongo (always — FR-3.4),
      (3) inserts a new `work_items` row OR bumps `signal_count` on an
      existing one in Postgres,
      (4) inserts a metric row into the Timescale hypertable.
- [x] Every sink write is retry-with-backoff (3 attempts, 100ms × 2). On
      exhaustion, the payload + error lands in the Mongo `dead_letter`
      collection (not auto-replayed in v1).
- [x] **Redis-down** → `/health` flips to `degraded` (status 200 because
      Redis is non-critical), debounce falls back to "always CREATED"
      (FR-3.6). On Redis restart, `*redis.Script.Run` auto-reloads the
      script on the first `NOSCRIPT` and debouncing resumes.
- [x] **Postgres-down** → work_item writes dead-letter after 3 retries;
      Mongo audit still receives the raw signals; backend keeps running.
- [x] `/health` pings every dep with a 500ms timeout and includes per-dep
      `{status, latency_ms}` in the response.
- [x] **Acceptance demo:** `./scripts/simulate-component-storm.sh` fires
      200 signals at one component over 8 seconds and verifies:
      ~200 raw signals in Mongo, 1–3 work_items in Postgres,
      200 rows in Timescale, **reduction ratio ≥ 60×**.
      Result: 2 work_items, **100× reduction**, 0 errors.

### Phase 3 env vars (added this phase)

| Var | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://ims:ims@localhost:5432/ims?sslmode=disable` | pgx pool DSN |
| `MONGO_URI` | `mongodb://ims:ims@localhost:27017/ims?authSource=admin` | mongo client URI |
| `MONGO_DATABASE` | `ims` | mongo logical database |
| `REDIS_ADDR` | `localhost:6379` | redis address |
| `IMS_DEBOUNCE_WINDOW_SECONDS` | `10` | FR-3.1 |
| `IMS_DEBOUNCE_MAX_SIGNALS` | `100` | FR-3.1 |
| `IMS_DEP_PING_TIMEOUT` | `500ms` | per-dep /health budget |

### What Phase 3 does *not* do

No state machine, no transitions, no RCA validation, no alerting.
The processor only ever creates work_items in `OPEN` state. Phase 4
adds the State pattern (Open → Investigating → Resolved → Closed),
the Strategy pattern for alerters (PagerDutyStub / Slack / Console),
the RCA model, and MTTR computation in `ClosedState.OnEnter`.

## Decisions log

Non-obvious choices are recorded in [`docs/decisions.md`](docs/decisions.md).
