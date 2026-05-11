# CLAUDE.md

This file gives Claude Code the operational context for working in this repository. It is auto-loaded on every session. Keep it short, dense, and current.

## What this project is

**Vellum — Distributed Signal and Incident Response System.** A backend-heavy distributed system that ingests failure signals at 10K/sec, debounces them into work items via atomic Redis Lua, runs them through a state-machine lifecycle with mandatory RCA on closure, and exposes a Next.js dashboard for triage. Showcases concurrency, distributed-systems design, and clean code.

For the full picture, read in order:
1. `docs/00-master-prd.md` — what we're building and why (functional + non-functional requirements)
2. `docs/01-architecture.md` — how it's built (runtime topology, design patterns, failure modes)
3. `docs/decisions.md` — every non-obvious choice, with rationale
4. `docs/prompts.md` — per-phase narrative of what was asked of Claude

**Never re-derive requirements. Always reference the PRD by section number.**

## Tech stack (non-negotiable)

| Layer | Choice | Notes |
|---|---|---|
| Backend lang | Go 1.22+ | idiomatic, small interfaces, no globals |
| HTTP | Gin | for `/v1/*` REST endpoints |
| RPC | gRPC + protobuf | server-streaming for ingestion |
| RDBMS | PostgreSQL 16 + TimescaleDB ext | source of truth + timeseries |
| NoSQL | MongoDB 7 | raw signal audit log |
| Cache | Redis 7 | debounce state + dashboard hot cache |
| pg driver | pgx v5 | NOT database/sql |
| Migrations | golang-migrate | SQL files in `backend/migrations/` |
| Frontend | Next.js 14 (App Router) | TypeScript |
| UI | shadcn/ui + Tailwind | no design-from-scratch |
| Orchestration | Docker Compose | single command bring-up |
| Load test | vegeta | scripts in `/scripts/` |

Do **not** swap out a stack item without explicit approval. If something feels missing, ask before adding.

## Repo layout

```
backend/
  cmd/vellum/main.go             # entrypoint; wires everything
  internal/
    ingest/      # HTTP + gRPC handlers, rate limit
    pipeline/    # bounded channel, worker pool, metrics ticker
    debounce/    # Lua script wrapper + fallback
    workflow/    # State pattern, transitions, RCA validation
    alert/       # Strategy pattern, alerters
    persist/
      pg/        # Postgres repo (pgx)
      mongo/     # Mongo repo
      redis/     # Redis client + Lua scripts
      timescale/ # TimescaleDB writer
    api/         # Gin routes, gRPC server
    model/       # Signal, WorkItem, RCA, State types
    obs/         # /health, metrics ticker, logging
  proto/         # .proto files + generated code
  migrations/    # golang-migrate SQL files
frontend/
  app/                          # Next.js App Router pages
  components/                   # shadcn/ui + composed
  lib/                          # api client
docker/
  compose.yaml                  # all services
  postgres/init.sql             # creates timescaledb extension
scripts/
  simulate-outage.go            # failure simulator
  load-test.sh                  # vegeta script
docs/                           # PRD, architecture, phases
```

## Code conventions

**Go:**
- `gofmt`-clean, `go vet`-clean, `golangci-lint run` passes
- Dependency injection via constructors. No package-level globals except metrics counters and the rate limiter (singleton).
- Small interfaces, defined where consumed (not where implemented)
- Errors wrapped with `fmt.Errorf("...: %w", err)` to preserve chain
- `context.Context` is always the first parameter on any function that does I/O or might be cancelled
- Run with `-race` always during dev; CI must pass `go test -race ./...`
- Test files live next to the code: `foo.go` + `foo_test.go`
- Use `t.Cleanup` for teardown, not `defer` inside subtests

**Go imports order (gofmt does this, but to be explicit):**
1. stdlib
2. third-party
3. internal (`github.com/<user>/vellum/internal/...`)

**TypeScript/Next.js:**
- App Router only (no Pages Router)
- Server Components by default; mark Client Components with `"use client"` only when needed (forms, real-time polling)
- Tailwind utility-first; no custom CSS files
- shadcn/ui components copied into `components/ui/` — don't reinvent

**SQL:**
- Snake_case for tables and columns
- Every table has `created_at`, `updated_at` (DEFAULT now())
- Every UUID column uses `uuid` type, not text
- Migrations are forward-only; never edit a committed migration

## The four critical design rules

These are not negotiable and Claude must enforce them on itself when writing code:

1. **Ingestion never blocks on persistence.** The HTTP/gRPC handler does a non-blocking send to the bounded channel and returns immediately. If the channel is full, return 503/ResourceExhausted. Never `time.Sleep`, never wait on a DB.

2. **State transitions are transactional.** A Work Item state change is: BEGIN, SELECT FOR UPDATE, run State pattern's `CanTransitionTo`, UPDATE work_items, INSERT state_transitions, COMMIT. All in one Postgres transaction with SERIALIZABLE isolation. No exceptions.

3. **RCA is required for CLOSED.** The `ResolvedState.CanTransitionTo(ClosedState, ctx)` method rejects with `ErrMissingRCA` or `ErrIncompleteRCA` if the RCA is missing or invalid. This rule lives in exactly one place. Do not duplicate it.

4. **Debounce is atomic via Lua.** The check-then-act on Redis MUST be a single Lua script execution, not separate `GET` and `SET` calls. The script lives in `backend/internal/debounce/script.lua` and is loaded at startup with `SCRIPT LOAD`.

## Common commands

```bash
# Bring up the whole stack
docker compose -f docker/compose.yaml up

# Run backend locally (assumes DBs are up via compose)
cd backend && go run ./cmd/vellum

# Run all tests with race detector
cd backend && go test -race ./...

# Run a single package's tests verbosely
cd backend && go test -race -v ./internal/workflow

# Generate protobuf code
cd backend && buf generate    # or: protoc --go_out=... --go-grpc_out=...

# Run load test against running backend
./scripts/load-test.sh

# Simulate cascading failure scenario
go run ./scripts/simulate-outage.go --target http://localhost:8080

# Frontend dev
cd frontend && pnpm dev

# Apply pending migrations
migrate -path backend/migrations -database "$DATABASE_URL" up
```

## Things to ask before doing

Ask the user (don't assume) when:
- A requirement seems to conflict with another
- A library version is ambiguous
- A new dependency would be added (always ask)
- Tests are failing for reasons that look intentional
- The scope of a phase file is unclear

Do **not** ask before:
- Running `gofmt`, `go vet`, `go test`
- Writing tests alongside code
- Adding logging or comments
- Standard refactors within a single file

## Output discipline

- When generating code: write the file, run the formatter, run the tests, then describe what changed in 2-3 sentences. Don't narrate every line.
- When explaining design: be concrete and tie back to PRD section numbers ("FR-3.5 in 00-master-prd").
- When uncertain: say so, propose two options, ask.

## Logging the decision trail

Append to `docs/decisions.md` after any of:
- Choosing between two viable approaches
- Deviating from the PRD (with justification)
- Discovering a bug that required a non-obvious fix
- Picking a library or version that wasn't in the stack table above

Format:
```
## YYYY-MM-DD — short title
**Context:** what problem
**Decision:** what we did
**Why:** rationale
**Alternatives considered:** what else we looked at
**Impact:** what this changes downstream
```

## What's out of scope (do not add)

- Authentication, SSO, RBAC
- Multi-tenancy
- Real PagerDuty/Datadog/Jira integration (stubs are fine)
- Kubernetes manifests, Helm charts
- Mobile app, email, SMS
- ML-based correlation or anomaly detection
- i18n, theming
- GDPR/retention controls

If you find yourself building any of the above, stop and ask.

## Resume-grade reminder

This project will be discussed in interviews. Every design decision must be defensible. When in doubt about a tradeoff, pick the option that has a cleaner explanation, not the one that looks fancier. If a senior reviewer asks "why did you do X?" — the answer should never be "because the AI suggested it."
