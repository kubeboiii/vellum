# STATE.md

> **Purpose:** Live status of the IMS build. Updated at the end of every working session.
> **For AI assistants:** Always read this file first. It tells you where the project is and what to do next. Do not assume; check here.

---

## Current state

**Phase:** 4 (Workflow Engine) — *complete, on branch `phase-4-workflow`*
**Last session ended:** 2026-05-11. Implemented the State pattern (`internal/workflow`) with four concrete states (Open/Investigating/Resolved/Closed); `ResolvedState.CanTransitionTo(ClosedState)` is the single enforcement point for the "RCA required" rule (CLAUDE.md rule 3). Transactional engine wraps each transition in a SERIALIZABLE Postgres tx with SELECT FOR UPDATE (rule 2). Implemented the Strategy pattern (`internal/alert`) with three concrete alerters (PagerDutyStub for P0, SlackWebhook for P1/P2 with HTTP-or-console fallback, ConsoleAlerter as P3 and last resort). Added migration 004 for the `rca` table with belt-and-braces CHECK constraints; new endpoints `PATCH /v1/incidents/:id/state` and `POST /v1/incidents/:id/rca` (compound RCA-insert + close in one tx). MTTR computation lives in `ClosedState.OnEnter`. Alerter dispatch is async per FR-6.4. Full PRD G3 happy-path + sad-path verified via curl: invalid transitions get 409, missing/incomplete RCA gets 422 with field details, complete RCA returns 201 with MTTR populated. `go test -race ./...` clean across 12 packages including a 2-goroutine concurrent-close test (exactly one wins).
**Next action:** Begin Phase 5 (gRPC + Frontend). First write `docs/phases/phase-5-grpc-frontend.md`. Then: define `backend/proto/signals.proto` for the gRPC streaming endpoint `SignalService.IngestSignals` (FR-1.2), generate Go code with `buf generate`, implement the gRPC server in `internal/ingest/grpc.go` that submits to the same `pipeline.Pipeline` as HTTP. Next.js dashboard: `frontend/app/page.tsx` live feed polling `/v1/incidents` every 2s, `frontend/app/incidents/[id]/page.tsx` detail page with state-transition controls, `frontend/app/incidents/[id]/rca/page.tsx` RCA submission form. Acceptance: dashboard renders 3 pages; live feed updates within 2s of a new signal; RCA form closes a work_item.

---

## Phase checklist

Tick boxes as phases complete. Each phase has acceptance criteria in its build-plan section that must be met before ticking.

- [x] **Phase 1 — Foundation** (Day 1): repo scaffolding, Docker Compose with all 4 DBs running, empty Go module, empty Next.js app, README skeleton
- [x] **Phase 2 — Ingestion & Backpressure** (Day 2): HTTP endpoint, bounded channel, worker pool, token-bucket rate limiter, `/health`, metrics ticker, **load test proves 10K signals/sec** *(verified: 600K req over 60s, 100% success, p99 1.89 ms)*
- [x] **Phase 3 — Debounce & Persistence Fan-out** (Day 3): Redis Lua debounce, Mongo raw signal writes, Postgres work-item writes, TimescaleDB metric writes, retry-with-backoff, dead-letter *(verified: 200 signals → 2 work_items, ratio 100×; Redis-restart resilience confirmed)*
- [x] **Phase 4 — Workflow Engine** (Day 4): State pattern, Strategy pattern (alerters), RCA model + validation, MTTR calculation, transactional state transitions, unit tests *(verified: PRD G3 end-to-end via curl + concurrent-close test passes)*
- [ ] **Phase 5 — gRPC + Frontend** (Day 5): gRPC streaming endpoint sharing pipeline, Next.js dashboard (live feed, detail, RCA form)
- [ ] **Phase 6 — Resilience & Simulation** (Day 6): failure simulator script, integration test, concurrency stress test, bug fixes
- [ ] **Phase 7 — Documentation & Polish** (Day 7): final README, architecture diagram refinement, demo video, decisions.md cleanup, dry-run from fresh clone

---

## Known issues / unresolved

*Things that are broken, ambiguous, or deferred. Resolve or accept before next phase.*

- (none yet)

---

## Open questions for the human

*Claude can drop questions here during a session if a decision needs human input but the human isn't around.*

- (none yet)

---

## Today's metrics (after relevant phases)

*Updated after Phase 2 (load test), Phase 3 (debounce ratio), Phase 6 (final).*

| Metric | Target | Achieved | When measured |
|---|---|---|---|
| Sustained ingestion | 10,000 sig/sec for 60s | **10,000/s, 0 dropped** | 2026-05-11 (Phase 2) |
| p99 ingestion latency | < 50ms | **1.89 ms** | 2026-05-11 (Phase 2) |
| Debounce reduction ratio | ≥ 60× (100 signals → 1 work item) | **100×** (200 signals → 2 work_items) | 2026-05-11 (Phase 3) |
| Unit test coverage (core pkgs) | ≥ 60% | **12 packages green** (workflow: 10 tests, alert: 8, model: 12, processor: 6, pg: 7 incl. concurrent-close) | 2026-05-11 (Phase 4) |
| `docker compose up` to healthy | < 90s | — | end Phase 7 |

---

## How to use this file

**End of each working session — update these things:**

1. Move the "Phase" line to current.
2. Update "Last session ended" with a one-line summary of what was completed.
3. Update "Next action" with the very first thing the next session should do.
4. Tick any newly-complete phase checkbox (only if its acceptance criteria are met).
5. Add any new known issues, open questions, or metric results.

**Start of each working session — paste to Claude:**

> Read `CLAUDE.md`, `STATE.md`, and the current phase section of `docs/build-plan-backend.md` (or `build-plan-frontend.md`). Confirm where we are. Then continue from STATE.md's "Next action."

That's the entire workflow. This file is small on purpose — it's the index, not the content.
