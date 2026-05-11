# IMS — Mission-Critical Incident Management System

A backend-heavy distributed system that ingests failure signals at **10K/sec**,
debounces them into work items, runs them through a state-machine lifecycle
with mandatory RCA on closure, and exposes a Next.js dashboard for triage.

Built as a 7-day engineering assignment. Currently at **Phase 1 (Foundation)**.

For the full picture, read in order:

1. [`docs/00-master-prd.md`](docs/00-master-prd.md) — what and why
2. [`docs/01-architecture.md`](docs/01-architecture.md) — how it's built
3. `docs/02-data-models.md` — schemas *(Phase 2+)*
4. `docs/03-api-contract.md` — endpoints *(Phase 2+)*
5. `docs/phases/phase-N-*.md` — day-by-day prompts *(TBD)*

---

## Quick start

Requires Docker (with Compose v2), Go 1.22+, Node 20+, and pnpm (via `corepack enable`).

```bash
# 1. Bring up the four data stores (Postgres+TimescaleDB, Mongo, Redis).
docker compose -f docker/compose.yaml up -d

# 2. Wait for healthchecks (~10s on a warm host).
docker compose -f docker/compose.yaml ps

# 3. Run the backend (Phase 1 = empty Gin server on :8080).
cd backend && go run ./cmd/ims
# -> curl http://localhost:8080/health  =>  {"phase":1,"status":"healthy"}

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

## Phase 1 acceptance

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

## What Phase 1 does *not* do

No application logic. No ingestion handlers, no debouncer, no workflow, no
persistence wiring. The internal package directories exist but are empty.
Phase 2 (Ingestion & Backpressure) adds the HTTP signal endpoint and the
bounded-channel worker pool.

## Decisions log

Non-obvious choices are recorded in [`docs/decisions.md`](docs/decisions.md).
