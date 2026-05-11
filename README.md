<h1 align="center">Vellum</h1>
<p align="center"><b>Distributed Signal and Incident Response System</b></p>

<p align="center">
  <a href="https://go.dev"><img alt="Go 1.22+" src="https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white"></a>
  <a href="https://nextjs.org"><img alt="Next.js 14" src="https://img.shields.io/badge/Next.js-14-000000?logo=next.js&logoColor=white"></a>
  <a href="https://grpc.io"><img alt="gRPC" src="https://img.shields.io/badge/gRPC-streaming-244c5a?logo=grpc&logoColor=white"></a>
  <a href="https://www.postgresql.org"><img alt="PostgreSQL 16" src="https://img.shields.io/badge/Postgres-16-4169E1?logo=postgresql&logoColor=white"></a>
  <a href="https://www.mongodb.com"><img alt="MongoDB 7" src="https://img.shields.io/badge/MongoDB-7-47A248?logo=mongodb&logoColor=white"></a>
  <a href="https://redis.io"><img alt="Redis 7" src="https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white"></a>
  <a href="https://www.timescale.com"><img alt="TimescaleDB" src="https://img.shields.io/badge/Timescale-hypertables-FDB515?logo=timescale&logoColor=white"></a>
  <img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-green">
</p>

<p align="center">
  Sustains <b>10,000 signals/sec</b> ingestion at <b>p99 &lt; 2ms</b>, collapses correlated failures <b>100× via atomic Redis Lua debounce</b>, and enforces post-mortem discipline through a state-machine workflow with a mandatory RCA gate. Polyglot persistence across four stores, two ingestion protocols (HTTP + gRPC), and a Next.js operator UI.
</p>

---

## The problem

Production systems do not fail quietly. A single Postgres hiccup cascades into thousands of error logs from every service that touches the database. In thirty seconds an SRE's pager can receive **5,000 alerts about one outage**.

Observability tools (Datadog, New Relic, Grafana) excel at *displaying* this firehose. They do not aggregate it. The human on call is asked to do triage by reading.

Vellum sits between raw signals and human responders. It turns 5,000 alerts about one outage into **one incident with 5,000 signals attached** — preserved for forensics, deduplicated for action, locked behind a state machine that cannot be marked closed without a structured post-mortem. Mean Time To Repair is computed automatically.

## What Vellum does

- **Ingests** signals at sustained 10K/sec over HTTP and gRPC-streaming, with token-bucket per-source rate limiting and bounded-channel backpressure that returns `503` in <5ms when overloaded.
- **Debounces** correlated signals atomically: a single Redis Lua script collapses up to 100 signals per `component_id` per 10-second window into one work item — safe under contention across replicas.
- **Persists** every raw signal to MongoDB for forensic audit, the aggregated work item to Postgres for transactional truth, and per-minute aggregates to TimescaleDB for the analytics dashboard. Failed writes retry with exponential backoff (3 attempts, base 100ms), then dead-letter to Mongo for human recovery.
- **Tracks lifecycle** through a State-pattern workflow (`OPEN → INVESTIGATING → RESOLVED → CLOSED`) under `SERIALIZABLE` isolation. CLOSED is unreachable without a complete RCA — the gate is enforced in exactly one place in the codebase, not duplicated. MTTR is auto-computed on close and written in the same transaction.
- **Alerts** through a Strategy-pattern registry: PagerDuty for P0, Slack webhooks for P1/P2, console for P3. Dispatch is async and isolated; a failing alerter cannot block ingestion.
- **Surfaces** all of this through a Next.js 14 operator dashboard: live feed, transition-timeline scrubber, signal-frequency histograms, payload-fingerprint grouping, MTTR distribution, root-cause donut, persona-aware view switcher.

## Benchmarks

Measured on a 2026 MacBook Air M4 against the full Docker Compose stack.

| Metric | Result | Source |
|---|---|---|
| Sustained ingestion throughput | **10,000 signals/sec for 60s** | `./scripts/load-test.sh` (vegeta) |
| Total requests served, zero drops | **600,000 over 60s, 100% success** | same |
| p99 ingestion latency | **1.89 ms** | same |
| Debounce compression ratio | **100×** (200 signals → 2 work items) | `simulate-outage.go --scenario cache` |
| Concurrency stress test | **300 goroutines × same component_id @ cap=100 → exactly 3 work items**, no races under `-race` | `internal/debounce/stress_test.go` |
| End-to-end lifecycle | **0.29s**: POST → debounce → state transitions → RCA gate (422) → CLOSED + MTTR | `internal/e2e_integration_test.go` |
| Frontend build | **11 routes prerendered**, 87 KB shared JS, zero TS errors | `pnpm build` |

## Architecture

```mermaid
flowchart LR
    src["Observability<br/>agents"] -- "HTTP / gRPC<br/>10K sig/s" --> ingest["Ingest<br/>(bounded channel<br/>+ rate limit)"]
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

Six invariants are load-bearing across the codebase:

1. **Ingestion never blocks on persistence.** Handler does a non-blocking `select { case ch <- sig: default: }` onto a `chan Signal` of capacity 50,000 and returns `202` (or `503` when full) in milliseconds. No request goroutine ever waits on a database — backpressure propagates back to the caller as `Retry-After: 1`.
2. **State transitions are transactional.** Every state change is `BEGIN → SELECT FOR UPDATE → CanTransitionTo() → UPDATE → INSERT audit → COMMIT` under `SERIALIZABLE` isolation. `pgx` `SQLSTATE 40001` maps to a retryable `409`.
3. **RCA is required for CLOSED.** `ResolvedState.CanTransitionTo(ClosedState, ctx)` is the only place the gate exists. Bypassing it is impossible by construction. MTTR is written in the same transaction.
4. **Debounce is atomic via Lua.** `GET window → INCR count → SET` runs as one Redis script execution, loaded at startup via `SCRIPT LOAD`. Multi-replica safe; verified under 300-goroutine `-race` contention.
5. **Persistence retries, then dead-letters.** Every sink write is wrapped in `cenkalti/backoff` (3 attempts, base 100ms, 2× multiplier). Final failures land in a Mongo `dead_letter` collection with the original payload and the sink that failed — recoverable later, never silently dropped.
6. **Concurrency primitives are explicit.** Goroutines + channels for the pipeline, `sync.RWMutex` for the per-source rate-limiter map, `atomic.Int64` for lock-free counters, `errgroup.Group` for the parallel persistence fan-out, `context.Context` on every I/O path. CI runs `go test -race` on every commit.

Full topology + design-pattern catalog in [`docs/01-architecture.md`](docs/01-architecture.md). 30+ ADR-style decision entries in [`docs/decisions.md`](docs/decisions.md).

## Observability

- **`GET /health`** — returns each dependency's status (`up` / `degraded` / `down`) with latency in ms, plus ingestion queue depth and capacity.
- **5-second metrics ticker** on the backend writes a structured line to stdout: `[metrics] accepted=8421/s processed=8398/s queue=312/50000 errors=0.00/s`.
- **Dashboard mirrors both live.** A dependency strip at the top of every page polls `/health` every 5s; the bottom of the live feed renders the metrics ticker in a terminal-styled frame.

## API surface

### HTTP

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/v1/signals` | Ingest a single signal — `202` accepted / `503` queue full |
| `GET` | `/v1/incidents` | Active (non-CLOSED) incidents, sorted by severity then last-signal time |
| `GET` | `/v1/incidents/closed` | Closed history with MTTR |
| `GET` | `/v1/incidents/:id` | Incident detail + RCA (if closed) |
| `GET` | `/v1/incidents/:id/signals` | Paginated raw signals from Mongo |
| `GET` | `/v1/incidents/:id/transitions` | State-transition audit log |
| `PATCH` | `/v1/incidents/:id/state` | Advance the state machine |
| `POST` | `/v1/incidents/:id/rca` | Submit RCA → automatic CLOSE + MTTR computation |
| `GET` | `/health` | Dependency status + queue depth |

### gRPC

`SignalService.IngestSignals` — bidi-streaming. Client streams `Signal` messages; server streams `Ack` (`ACCEPTED` / `REJECTED_QUEUE_FULL` / `REJECTED_INVALID`). Shares the same downstream pipeline as HTTP.

## Tech stack — and why

| Layer | Choice | Why this and not something else |
|---|---|---|
| **Backend language** | Go 1.22+ | Goroutines and channels map cleanly to the ingestion model. Static binary, fast cold start, race detector in the test runner. |
| **HTTP framework** | Gin | Lightweight, ergonomic middleware composition, low allocation in the hot path. Hardened with `SetTrustedProxies(nil)`, `MaxBytesReader`, and full HTTP-server timeouts. |
| **RPC** | gRPC + protobuf | Strongly-typed contract for high-volume internal producers. Server-streaming endpoint shares the HTTP pipeline downstream. |
| **Source-of-truth store** | PostgreSQL 16 | Work items have foreign keys to RCAs and transitions. `SERIALIZABLE` prevents split-brain transitions. |
| **Audit log store** | MongoDB 7 | Signal payloads are heterogeneous JSON; Mongo is purpose-built for append-only, schema-flexible, secondary-indexed writes at volume. |
| **Cache + debounce state** | Redis 7 | (a) Atomic Lua scripts — the single primitive that makes correct cross-replica debounce possible. (b) Dashboard hot-path: sorted set keyed by `(severity, last_signal_ts)` gives O(log n) reads, avoiding a Postgres round-trip on every 2s poll. |
| **Timeseries** | TimescaleDB (Postgres extension) | Hypertables auto-partition by time for per-minute rollups. One less container than running Prometheus. |
| **Postgres driver** | `pgx` v5 | Native protocol, built-in pooling, structured-error access for SQLSTATE-based retry logic. Not `database/sql`. |
| **Frontend** | Next.js 14 (App Router) + Tailwind + hand-rolled components | Server Components for static pages, Client Components for the polling dashboard. Owned design system per a checked-in `THEME.md`. |
| **Orchestration** | Docker Compose | Single command brings the whole stack up. Pinned image digests. |
| **Load test** | vegeta | Scriptable, clean reports, scales past 10K rps from one host. |

## Prerequisites

| Tool | Version | Install (macOS) |
|---|---|---|
| Docker (with Compose v2) | latest | `brew install --cask docker` |
| Go | 1.22+ | `brew install go` |
| Node + pnpm | Node 20+, pnpm via corepack | `brew install node && corepack enable` |
| golang-migrate | latest | `brew install golang-migrate` |

Linux: install via your package manager, [go.dev/dl](https://go.dev/dl/), [nodesource](https://github.com/nodesource/distributions), and the [migrate releases page](https://github.com/golang-migrate/migrate/releases).

## Getting started

```bash
# 1. Clone + configure
git clone https://github.com/kubeboiii/vellum.git
cd vellum
cp .env.example .env

# 2. Start Postgres, MongoDB, Redis (all bound to 127.0.0.1)
docker compose -f docker/compose.yaml up -d
docker compose -f docker/compose.yaml ps       # wait until all three show (healthy)

# 3. Apply schema migrations
export DATABASE_URL="postgres://vellum:vellum@localhost:5432/vellum?sslmode=disable"
migrate -path backend/migrations -database "$DATABASE_URL" up

# 4. Run the backend (HTTP :8080, gRPC :9090)
cd backend && go run ./cmd/vellum
```

Verify and send a signal:

```bash
curl -s http://localhost:8080/health | jq
curl -sX POST http://localhost:8080/v1/signals \
  -H 'Content-Type: application/json' \
  -d '{"component_id":"RDBMS_PRIMARY_01","component_type":"RDBMS","severity":"P0","source":"datadog","payload":{"err":"connection refused"}}'
# → 202 {"signal_id":"...","status":"accepted"}
```

In a second terminal, start the frontend at [localhost:3000](http://localhost:3000):

```bash
cd frontend
pnpm install                                   # first time only
pnpm dev                                       # or `pnpm build && pnpm start`
```

### 60-second demo

Open [`/dashboard`](http://localhost:3000/dashboard) and in a terminal run:

```bash
go run ./scripts/simulate-outage.go --scenario all     # ~460 signals over ~15s → ~9 incidents
./scripts/load-test.sh                                 # sustained 10K/sec for 60s (needs `brew install vegeta`)
```

The live feed updates within 2 seconds. Click any incident for its transition timeline, signal-frequency histogram, payload-fingerprint groups. Submit an RCA to watch the incident close with computed MTTR; view it on [`/incidents/closed`](http://localhost:3000/incidents/closed) inside the MTTR distribution chart.

Tear down with `docker compose -f docker/compose.yaml down -v`.

## Repository layout

```
vellum/
├── backend/
│   ├── cmd/vellum/                    # main entrypoint; wires the pipeline
│   ├── internal/
│   │   ├── ingest/                    # HTTP handler + gRPC streaming + rate limit
│   │   ├── pipeline/                  # bounded channel + worker pool
│   │   ├── debounce/                  # Lua script + atomic wrapper
│   │   ├── workflow/                  # State pattern; transactional transitions
│   │   ├── alert/                     # Strategy registry; PagerDuty / Slack / Console
│   │   ├── persist/{pg,mongo,redis,timescale}/
│   │   ├── api/                       # Gin routes for the dashboard
│   │   ├── model/                     # pure types: Signal, WorkItem, RCA, State (+ unit tests)
│   │   ├── obs/                       # /health, 5s metrics ticker
│   │   └── processor/                 # consumes the channel; orchestrates fan-out + retry
│   ├── proto/vellum/v1/               # protobuf + generated stubs
│   └── migrations/                    # forward-only SQL migrations
├── frontend/                          # Next.js 14 dashboard + landing + demo pages
├── docker/compose.yaml                # Postgres + Mongo + Redis, 127.0.0.1-bound
├── scripts/
│   ├── simulate-outage.go             # headless failure simulator (3 scenarios)
│   ├── load-test.sh                   # vegeta 10K/sec load test
│   └── grpc-client.go                 # demo gRPC streaming client
└── docs/
    ├── 00-master-prd.md               # requirements
    ├── 01-architecture.md             # topology + design patterns + failure modes
    ├── decisions.md                   # 30+ ADRs
    └── prompts.md                     # AI-assisted process narrative
```

## Operator UI

| Route | Purpose |
|---|---|
| `/` | Landing page |
| `/dashboard` | Live feed, severity mix, noisiest components, persona switcher, health strip |
| `/incidents/[id]` | Detail: transition timeline, time-in-each-state, frequency histogram, fingerprints, raw signals |
| `/incidents/[id]/rca` | RCA submission with client + server validation |
| `/incidents/closed` | History: MTTR distribution, root-cause donut, repeat-offenders, MTTR trend |
| `/postmortem` | RESOLVED queue waiting for RCAs + quality stats |
| `/simulate` · `/load-test` | Click-to-run failure scenarios + in-browser burst tester |

## Testing

```bash
cd backend && go test -race ./...                            # all unit + concurrency tests
go test -tags=integration -race ./internal/                  # E2E lifecycle (backend on :8080)
cd frontend && pnpm build                                    # TS check + prerender
govulncheck -mode=source ./...                               # zero findings as of latest commit
```

RCA validation has dedicated unit tests in `internal/model/rca_test.go`; the workflow's RCA gate has a dedicated integration test asserting `RESOLVED → CLOSED` returns `422` without a complete RCA body.

## Security posture

Default credentials (`vellum:vellum`) and plaintext HTTP are intentional for local dev; DB ports bind to `127.0.0.1` only. The system is hardened against trivial attacks (`http.MaxBytesReader`, `SetTrustedProxies(nil)`, full HTTP server timeouts, `govulncheck`-clean dependencies). No secrets in repo or git history — verified via full-history pattern scan.

## Conscious non-goals

No authentication (schema is auth-ready: `actor` and `submitted_by` wait for verified identity). No multi-tenancy. No real PagerDuty/Datadog integration — the Strategy pattern + stubs prove the contract; swapping a stub for a real HTTP client is a one-file change. No Kubernetes; Docker Compose is the unit of deployment. No ML correlation; debounce is rule-based on `(component_id, time, count)`. Every omission is recorded with rationale in [`docs/decisions.md`](docs/decisions.md).

## License

MIT. See [`LICENSE`](LICENSE).

