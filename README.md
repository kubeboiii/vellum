# Vellum — Mission-Critical Incident Management System

> **Ingests failure signals at 10K/sec, debounces them into work items via atomic Redis Lua, runs them through a state-machine lifecycle with mandatory RCA on closure, and surfaces them through a Next.js triage dashboard.**

A 7-day backend engineering project demonstrating high-throughput ingestion with backpressure, atomic debounce, polyglot persistence, State + Strategy design patterns, and a polished operator UI. **All 7 phases complete.**

```mermaid
flowchart LR
    src["Observability<br/>agents"] -- "HTTP / gRPC<br/>10k/s" --> ingest["Ingest<br/>(bounded channel<br/>+ rate limit)"]
    ingest --> debounce["Debouncer<br/>(Redis Lua<br/>atomic)"]
    debounce --> wf["Workflow<br/>(State pattern<br/>+ RCA gate)"]
    wf --> stores[("<b>Postgres + Mongo<br/>+ Redis + Timescale</b>")]
    wf -- "Strategy<br/>fanout" --> alert["Alerters<br/>(PagerDuty / Slack<br/>/ Console)"]
    wf -- "polled" --> dash["Next.js<br/>dashboard"]

    classDef proc fill:#0a0a0a,stroke:#bef264,stroke-width:1.5px,color:#d4d4d8
    classDef store fill:#0a0a0a,stroke:#a78bfa,stroke-width:1.5px,color:#d4d4d8
    class ingest,debounce,wf proc
    class stores store
```

For the full picture, read in order:

1. [`docs/00-master-prd.md`](docs/00-master-prd.md) — what and why
2. [`docs/01-architecture.md`](docs/01-architecture.md) — how it's built (with detailed diagrams)
3. [`docs/decisions.md`](docs/decisions.md) — every non-obvious choice, with rationale
4. [`docs/prompts.md`](docs/prompts.md) — per-phase narrative of what was asked of Claude
5. [`docs/phases/`](docs/phases/) — day-by-day specs for each phase

---

## Quick start

Requires Docker (with Compose v2), Go 1.22+, Node 20+, pnpm (via `corepack enable`), and the golang-migrate CLI (`brew install golang-migrate`).

```bash
# 1. Bring up the four data stores (Postgres+TimescaleDB, Mongo, Redis).
docker compose -f docker/compose.yaml up -d

# 2. Wait for healthchecks (~10s on a warm host).
docker compose -f docker/compose.yaml ps

# 3. Apply SQL migrations (creates work_items, state_transitions, signal_metrics hypertable).
export DATABASE_URL="postgres://vellum:vellum@localhost:5432/vellum?sslmode=disable"
migrate -path backend/migrations -database "$DATABASE_URL" up

# 4. Run the backend on :8080.
cd backend && go run ./cmd/vellum
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
backend/   Go service (cmd/vellum, internal/{ingest,pipeline,debounce,workflow,...})
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
- [x] `cd backend && go run ./cmd/vellum` starts a Gin server on `:8080`.
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
      then the pipeline drains within `VELLUM_SHUTDOWN_TIMEOUT` (default 30s).
- [x] **Load test:** `./scripts/load-test.sh` reports 10,000 req/s sustained
      for 60s, 100% success, p99 = 1.89 ms (target ≤ 50 ms), 0 dropped.

### Phase 2 config (env vars, with defaults)

| Var | Default | Purpose |
|---|---|---|
| `VELLUM_HTTP_ADDR` | `:8080` | bind address |
| `VELLUM_QUEUE_CAPACITY` | `50000` | bounded-channel depth (~5s of nominal at 10K/s) |
| `VELLUM_WORKER_COUNT` | `NumCPU()*2` | consumer goroutines |
| `VELLUM_RATE_LIMIT_RPS` | `1000` | per-source token refill rate (FR-1.6) |
| `VELLUM_RATE_LIMIT_BURST` | `2000` | per-source burst tolerance |
| `VELLUM_METRICS_INTERVAL` | `5s` | stdout metrics cadence (FR-8.2) |
| `VELLUM_SHUTDOWN_TIMEOUT` | `30s` | drain deadline (NFR-2.4) |

### Running the load test yourself

```bash
# Terminal 1 — boot the backend with rate limit lifted for single-host benchmark
cd backend && VELLUM_RATE_LIMIT_RPS=20000 VELLUM_RATE_LIMIT_BURST=40000 go run ./cmd/vellum

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
| `DATABASE_URL` | `postgres://vellum:vellum@localhost:5432/vellum?sslmode=disable` | pgx pool DSN |
| `MONGO_URI` | `mongodb://vellum:vellum@localhost:27017/vellum?authSource=admin` | mongo client URI |
| `MONGO_DATABASE` | `vellum` | mongo logical database |
| `REDIS_ADDR` | `localhost:6379` | redis address |
| `VELLUM_DEBOUNCE_WINDOW_SECONDS` | `10` | FR-3.1 |
| `VELLUM_DEBOUNCE_MAX_SIGNALS` | `100` | FR-3.1 |
| `VELLUM_DEP_PING_TIMEOUT` | `500ms` | per-dep /health budget |

## Phase 4 acceptance (Workflow Engine)

- [x] Migration 004 creates the `rca` table with DB-level CHECK
      constraints mirroring app-level `RCA.Validate()`.
- [x] State pattern in `internal/workflow`: `OpenState`,
      `InvestigatingState`, `ResolvedState`, `ClosedState`, each
      implementing `State` with `Name`, `CanTransitionTo`, `OnEnter`.
- [x] `ResolvedState.CanTransitionTo(ClosedState)` is the **single**
      enforcement point for "RCA required to close" (CLAUDE.md rule 3).
- [x] `ClosedState.OnEnter` computes MTTR and stamps it on the WI.
- [x] Transitions run in a SERIALIZABLE pgx tx with
      `SELECT FOR UPDATE` (CLAUDE.md rule 2 + FR-4.4).
- [x] Strategy pattern in `internal/alert`: `PagerDutyStub` (P0,
      logs structured JSON), `SlackAlerter` (P1/P2, HTTP POST if
      `SLACK_WEBHOOK_URL` set, else console), `ConsoleAlerter` (P3 +
      fallback). One-file adds a new alerter.
- [x] Alert dispatch on CREATED is async (`go alerter.Dispatch(...)`),
      5s timeout, errors logged not dead-lettered (FR-6.4).
- [x] HTTP endpoints (`internal/api`):
      `GET /v1/incidents`, `GET /v1/incidents/:id`,
      `PATCH /v1/incidents/:id/state`,
      `POST /v1/incidents/:id/rca` (compound RCA-insert + close in
      one tx).
- [x] `go test -race ./...` clean across **12 packages**, including
      `TestEngine_ConcurrentClose_ExactlyOneWins` (2 goroutines try to
      close the same WI; exactly one succeeds).
- [x] **PRD G3 end-to-end** (PATCH to CLOSED with no RCA → 422; with
      short RCA → 422 with field details; with complete RCA → 201
      with `mttr_seconds` populated). Verified via curl against a
      live stack.

### Phase 4 env vars (added this phase)

| Var | Default | Purpose |
|---|---|---|
| `SLACK_WEBHOOK_URL` | (empty) | If set, P1/P2 alerts HTTP POST here. Else Console. |
| `VELLUM_ALERTER_TIMEOUT` | `5s` | Per-dispatch timeout (FR-6.4). |

### Try the PRD G3 scenario yourself

```bash
# Boot the stack + run migrations + start the backend (Phase 3 steps).

# 1. Fire a signal to create a P0 incident (PagerDuty stub fires).
curl -X POST -H 'Content-Type: application/json' http://localhost:8080/v1/signals \
  -d '{"component_id":"DEMO_1","component_type":"CACHE","severity":"P0","source":"manual","payload":{}}'

# 2. List active incidents and grab the ID.
curl http://localhost:8080/v1/incidents | jq '.items[0].id'

# 3. Advance through the lifecycle.
WI_ID=<paste-id>
curl -X PATCH -H 'Content-Type: application/json' http://localhost:8080/v1/incidents/$WI_ID/state \
  -d '{"to":"INVESTIGATING"}'
curl -X PATCH -H 'Content-Type: application/json' http://localhost:8080/v1/incidents/$WI_ID/state \
  -d '{"to":"RESOLVED"}'

# 4. Try to close without RCA → 422.
curl -X PATCH -H 'Content-Type: application/json' http://localhost:8080/v1/incidents/$WI_ID/state \
  -d '{"to":"CLOSED"}'

# 5. Submit a complete RCA → 201 with MTTR.
curl -X POST -H 'Content-Type: application/json' http://localhost:8080/v1/incidents/$WI_ID/rca \
  -d '{
    "root_cause_category":"INFRASTRUCTURE",
    "fix_applied":"Rebooted the cache cluster and bumped pool size.",
    "prevention_steps":"Add a synthetic monitor for pool saturation.",
    "submitted_by":"sre@example.com"
  }'
```

### What Phase 4 does *not* do

No gRPC ingestion, no frontend. The new API endpoints exist for the
Phase 5 dashboard to consume; the dashboard itself ships in Phase 5.

## Phase 5 acceptance (gRPC + Frontend)

- [x] `backend/proto/vellum/v1/signals.proto` defines bidi-stream
      `SignalService.IngestSignals`. `buf generate` (local plugins)
      produces `signals.pb.go` + `signals_grpc.pb.go`.
- [x] gRPC server on `:9090` shares the same `pipeline.Pipeline` as
      HTTP (FR-1.3 — "exactly one downstream path regardless of
      protocol"). 4 bufconn-backed unit tests cover ACCEPTED, queue-full,
      invalid signal, client-supplied UUID preservation.
- [x] `GET /v1/incidents/:id/signals?page=&per_page=` paginates raw
      signals from Mongo. UUIDs returned as canonical strings, not
      BSON Binary.
- [x] CORS middleware (~30 LOC, no extra dep) lets the dashboard
      call the API from `localhost:3000` during dev.
- [x] **SQLSTATE 40001 → 409 Conflict** mapping in the API error
      helper. Concurrent processor + workflow tx don't surface as
      scary 500s.
- [x] Next.js 14 dashboard with three pages:
      `/` live feed (2s client-side polling, FR-7.1),
      `/incidents/[id]` detail with state-transition controls + raw
      signals + RCA panel (FR-7.2, FR-7.4),
      `/incidents/[id]/rca` RCA form with client-side validation
      mirror (FR-7.3).
- [x] `pnpm build` clean: 4 routes, ~98 KB First Load JS.
- [x] `go test -race ./...` clean across **13 packages** (incl. 4 new
      gRPC tests). `go vet`, `gofmt` clean.
- [x] **End-to-end smoke:** 10 signals streamed via gRPC →
      work_item created → PATCH OPEN→INVESTIGATING→RESOLVED via
      REST → POST RCA returns 201 with `mttr_seconds` populated.
      PagerDuty stub fires for the gRPC-originated P0.

### Phase 5 env vars (added this phase)

| Var | Default | Purpose |
|---|---|---|
| `VELLUM_GRPC_ADDR` | `:9090` | gRPC bind address |
| `VELLUM_CORS_ORIGINS` | `http://localhost:3000` | Comma-separated allowed Origins |
| `NEXT_PUBLIC_API_BASE` | `http://localhost:8080` | Frontend → backend URL (build-time, exposed to browser) |

### Run the full system

```bash
# 1. Bring up the data stores + apply migrations.
docker compose -f docker/compose.yaml up -d
export DATABASE_URL="postgres://vellum:vellum@localhost:5432/vellum?sslmode=disable"
migrate -path backend/migrations -database "$DATABASE_URL" up

# 2. Start the backend (HTTP :8080 + gRPC :9090).
cd backend && go run ./cmd/vellum

# 3. In another terminal — start the dashboard.
cd frontend && pnpm dev
# -> open http://localhost:3000

# 4. Optional — stream signals over gRPC to populate the dashboard.
cd backend && go run -tags phase5demo ../scripts/grpc-client.go \
  --target localhost:9090 --n 20 --component DEMO_GRPC
```

### Regenerating gRPC stubs

After editing `backend/proto/vellum/v1/signals.proto`:

```bash
cd backend
PATH="$HOME/go/bin:$PATH" buf generate
```

Generated files are checked into git; reviewers don't need protoc
installed unless they want to modify the .proto. Required tooling for
that:

```bash
brew install bufbuild/buf/buf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### What Phase 5 does *not* do

No failure-simulator script, no chaos tests, no WebSocket push.
The cascading-failure scenario from PRD §7 G6 ships in Phase 6
(`scripts/simulate-outage.go`), along with an integration test that
exercises the entire stack against ephemeral containers.

## Phase 6 acceptance (Resilience & Simulation)

| Deliverable | Where | Verification |
|---|---|---|
| **Concurrency stress test** | [`backend/internal/debounce/stress_test.go`](backend/internal/debounce/stress_test.go) | 300 goroutines × same `component_id` at the production cap (100) → exactly `ceil(300/100)=3` distinct work_items, 3 CREATED actions. Passes under `-race`. |
| **Boundary-arithmetic test** | same file | Sequential 101 signals → first 100 JOIN, 101st CREATEs. Deterministic. |
| **E2E integration test** | [`backend/internal/e2e_integration_test.go`](backend/internal/e2e_integration_test.go) | Behind the `integration` build tag; exercises POST signal → debounce → state machine → RCA gate (422) → CLOSED + MTTR computed → transitions audit. Runs in 0.29s. |
| **Headless failure simulator** | [`scripts/simulate-outage.go`](scripts/simulate-outage.go) | Three scenarios (rdbms / cache / mcp) producing exact debounce ratios per scenario. |

### Try the PRD G2 scenario yourself

```bash
# Cache thrash: 200 P2 signals to one component, debouncer must collapse to 2.
go run ./scripts/simulate-outage.go --scenario cache
#  → 200 sent, 200 accepted, 2 work_items created, ratio 100.0×
#  → ✓ debounce held exactly as predicted
```

```bash
# Run the integration suite against the live backend on :8080.
go test -tags=integration -race ./backend/internal/
#  → PASS: TestE2E_FullLifecycle (0.29s)
```

### What Phase 6 does *not* do

No new endpoints. No schema changes. Just stress + simulation.

## Phase 7 acceptance (Documentation & Polish)

The final pass. No new code, just making everything reviewable.

| Deliverable | Where |
|---|---|
| Final README (this file) | top-level Mermaid pitch + all 7 phase sections + how-to-demo |
| Mermaid architecture diagrams | [`docs/01-architecture.md`](docs/01-architecture.md) §2, §3; one inlined here |
| `docs/prompts.md` | [`docs/prompts.md`](docs/prompts.md) — per-phase narrative of what Claude was asked, what worked, what I pushed back on |
| `decisions.md` final entries | [`docs/decisions.md`](docs/decisions.md) — 27 entries spanning Phases 1–7 |
| Dry-run from fresh clone | verified: `docker compose up` → backend healthy → `go test -race ./...` clean → `pnpm build` clean → simulator scenarios match predictions |

## How to demo

Three commands prove the three PRD headline goals (§7):

```bash
# G1: Sustain 10K signals/sec for 60s, p99 < 50ms.
./scripts/load-test.sh
# expect: ≥99% 2xx, p99 ≤ 50ms, queue depth never blocks

# G2: Debounce collapses correlated signals 100x.
go run ./scripts/simulate-outage.go --scenario cache
# expect: 200 sent → 2 work_items → ratio 100.0×

# G3: RCA enforcement is unbypassable.
go test -tags=integration -race ./backend/internal/
# expect: TestE2E_FullLifecycle PASS, including 422 on CLOSED without RCA
```

Then open the dashboard at [http://localhost:3000/dashboard](http://localhost:3000/dashboard) and click through:
- Live feed with persona switcher
- Click any incident → transition timeline, signal frequency histogram, payload fingerprints
- [`/postmortem`](http://localhost:3000/postmortem) for the RCA queue
- [`/incidents/closed`](http://localhost:3000/incidents/closed) for MTTR distribution + root-cause donut

## Decisions log

Non-obvious choices are recorded in [`docs/decisions.md`](docs/decisions.md).
