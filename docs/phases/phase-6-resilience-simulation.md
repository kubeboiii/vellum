# Phase 6 — Resilience & Simulation

> **Document:** `docs/phases/phase-6-resilience-simulation.md`
> **Owner:** Backend — Phase 6
> **Created:** 2026-05-11
> **PRD references:** §13 (build phases), §5.2 (resilience), §10 R2 (concurrency-bug risk), §7 (success criteria)

## 0. Why this phase exists

By the end of Phase 5 we have a complete end-to-end system: gRPC + HTTP ingest, debouncer, state machine, RCA gate, dashboard. Phase 6 stops adding features and **stress-tests what we already built**. Three concrete deliverables:

1. **A headless failure simulator** (`scripts/simulate-outage.go`) that a reviewer can run from a clean checkout to see the system handle a realistic cascading outage.
2. **An end-to-end integration test** that exercises the entire signal-to-RCA lifecycle against ephemeral containers.
3. **A concurrency stress test** that proves the single hardest property: N goroutines firing the same `component_id` produce **exactly one** work_item (PRD risk R2).

Plus a **bug-fix pass** — whatever the above turn up, we fix.

## 1. Scope — IN

| Deliverable | What it must demonstrate |
|---|---|
| `scripts/simulate-outage.go` | Cascading RDBMS → cache → MCP-host scenario, fires from CLI, prints a one-page summary at the end ("sent X signals, backend created Y work items, debounce ratio Z×") |
| `backend/internal/integration_test.go` (or per-package counterpart) | Full lifecycle test against real DBs (ephemeral via testcontainers OR docker-compose-launched, pick one) |
| `backend/internal/debounce/concurrency_test.go` | N goroutines × same component_id → assert exactly one work_item via Postgres |
| Bug-fix pass | Anything the above expose. Document each fix in `docs/decisions.md` |

## 2. Scope — OUT (do not do this pass)

- Adding new endpoints
- Adding new frontend features (Phase 5 is done)
- WebSocket / SSE upgrades (Tier 3, future)
- Schema changes
- Real PagerDuty / Slack-at-scale integration

## 3. The failure simulator

### Behavior
- CLI: `go run ./scripts/simulate-outage.go --target http://localhost:8080`
- Three scenarios, runnable individually or sequentially:
  - `--scenario rdbms` — 50 P0 to `RDBMS_PRIMARY_01`, then 100 P1 to `API_CHECKOUT`
  - `--scenario cache` — 200 P2 to `CACHE_CLUSTER_A`
  - `--scenario mcp` — 30 P0 to `MCP_HOST_INDEXER` + 80 P1 fanned across 4 APIs
  - `--scenario all` (default) — runs all three, sequentially
- After each scenario: poll `/v1/incidents?limit=500`, count work_items by component, print:
  ```
  scenario: rdbms-cascade
    sent:       150 signals (50 P0 + 100 P1)
    accepted:   150 (100.0%)
    rejected:   0 (0.0%)
    duration:   3.0s
    backend:    2 work_items created
    ratio:      75× debounce compression
  ```
- One-page summary at the very end aggregating everything.

### Implementation outline
- Single file, ~250 lines
- Use stdlib `net/http`, no external deps
- Parallel goroutine pool per scenario, paced via `time.Sleep`
- Defensive: tolerate 503s (count them) and 500s (log them), don't crash

### Why this script in addition to the `/simulate` page?
PRD G2 demos require a headless artifact a reviewer can run on a fresh checkout without spinning up the frontend. The script is also the canonical demo for a recorded demo video.

## 4. The end-to-end integration test

### Behavior
- Lives in `backend/internal/...` (we'll pick the package; probably its own `internal/integration_test.go`)
- Spins up real Postgres + Mongo + Redis (testcontainers if available, otherwise assumes docker-compose is up via build tag)
- Runs in CI under `go test -tags=integration ./...` so unit tests stay fast
- Exercises:
  1. POST a signal → assert 202
  2. Assert work_item row appears in Postgres
  3. Assert raw signal stored in Mongo
  4. PATCH state OPEN → INVESTIGATING → RESOLVED
  5. Try PATCH RESOLVED → CLOSED → assert **422**
  6. POST RCA → assert work_item.status=CLOSED, mttr_seconds > 0
  7. Verify state_transitions audit log has 3 rows in order

### Acceptance
- Runs in under 30 seconds
- Passes 10× in a row without flake

## 5. The concurrency stress test

### Behavior
- Lives at `backend/internal/debounce/concurrency_test.go`
- 50 goroutines fire 200 signals each to the SAME `component_id`, all within 1 second
- After the burst completes, query Postgres for work_items with that component_id
- Assert: **exactly ⌈10000 / 100⌉ work_items exist** (each window holds at most 100 signals)

### Why this test is critical
This is the PRD R2 risk: "Concurrency bugs hide in tests." Without this test, a race in the Lua script wrapper or the work_item-creation path could go undetected until production. With it, we have one hard property nailed down forever.

### Acceptance
- Passes with `go test -race`
- 10 consecutive runs without flake

## 6. Bug-fix pass

Run all of the above. Fix:
- Any race conditions surfaced
- Any 5xx errors under load
- Any incorrect MTTR / signal_count arithmetic
- Any frontend pages that crash on edge-case data the simulator produces

Document each non-trivial fix as a `decisions.md` entry.

## 7. Build order

1. **Concurrency stress test** — fastest feedback loop, highest information value, smallest scope
2. **End-to-end integration test** — locks in the lifecycle properties
3. **Simulator script** — wraps the demonstration narrative
4. **Bug-fix pass** — fix whatever broke
5. **Commit + PR**

## 8. Verification

| Check | Pass criteria |
|---|---|
| `go test -race ./...` | All packages, including the new concurrency test |
| `go test -tags=integration ./...` | Integration test passes 10× consecutively |
| `go run ./scripts/simulate-outage.go --scenario all` | Summary prints, debounce ratio visible per scenario |
| Frontend still works | `pnpm build` clean, all 9 routes 200 |
| No new `docs/decisions.md` entry left out | Every non-trivial bug fix recorded |

## 9. Acceptance criteria (PRD §7)

This phase closes G2 (debounce ratio ≥ 60×) under stress and proves R2 mitigation. Specifically:
- The concurrency test mathematically proves the debouncer holds under contention
- The simulator generates a debounce ratio of ≥75× per scenario
- The integration test proves the RCA gate is unbypassable from any client

## 10. Out-of-scope reminders

- Don't refactor handlers
- Don't change the schema
- Don't add gRPC clients beyond what already exists
- Don't touch the frontend except to fix bugs the simulator triggers
