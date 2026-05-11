# STATE.md

> **Purpose:** Live status of the IMS build. Updated at the end of every working session.
> **For AI assistants:** Always read this file first. It tells you where the project is and what to do next. Do not assume; check here.

---

## Current state

**Phase:** 3 (Debounce & Persistence Fan-out) — *complete, on branch `phase-3-debounce`*
**Last session ended:** 2026-05-11. Built the four persistence layers (`internal/persist/{pg,mongo,redis,timescale}`), the atomic Redis Lua debouncer (`internal/debounce` + `script.lua`), and the orchestrating processor (`internal/processor`). SQL migrations create `work_items`, `state_transitions`, and the TimescaleDB `signal_metrics` hypertable. Wired everything into `cmd/ims/main.go`: pgx pool, Mongo client, Redis client with SCRIPT LOAD, processor injected into the pipeline. `/health` now pings every dep with a 500ms timeout. Acceptance demo (`scripts/simulate-component-storm.sh`): 200 signals to one component → 2 work_items, 200 raw signals in Mongo, 200 rows in Timescale, **reduction ratio 100×** (target ≥ 60×). Redis-restart resilience verified: killing Redis flips status to `degraded` and the fallback creates a work_item per signal; on restart, NOSCRIPT auto-reload via `*redis.Script.Run` resumes normal debouncing. All packages have tests (testcontainers for repo tests, fakes for the processor). `go test -race ./...` clean across 10 packages.
**Next action:** Begin Phase 4 (Workflow Engine). First write `docs/phases/phase-4-workflow.md`. Then: implement `internal/workflow` with the State pattern (Open/Investigating/Resolved/Closed), `internal/alert` with the Strategy pattern (PagerDutyStub for P0, SlackWebhook for P1/P2, Console fallback). Add the RCA model + validation, the MTTR computation in `ClosedState.OnEnter`, and the transactional state-transition wrapper (SERIALIZABLE isolation + SELECT FOR UPDATE in Postgres). Wire alerters into the processor's `CREATED` branch so new work_items fire alerts. Acceptance: unit tests cover every transition path + every rejection (incl. ErrMissingRCA, ErrInvalidTransition); a PATCH /state endpoint advances a work_item and a PATCH /rca closes it.

---

## Phase checklist

Tick boxes as phases complete. Each phase has acceptance criteria in its build-plan section that must be met before ticking.

- [x] **Phase 1 — Foundation** (Day 1): repo scaffolding, Docker Compose with all 4 DBs running, empty Go module, empty Next.js app, README skeleton
- [x] **Phase 2 — Ingestion & Backpressure** (Day 2): HTTP endpoint, bounded channel, worker pool, token-bucket rate limiter, `/health`, metrics ticker, **load test proves 10K signals/sec** *(verified: 600K req over 60s, 100% success, p99 1.89 ms)*
- [x] **Phase 3 — Debounce & Persistence Fan-out** (Day 3): Redis Lua debounce, Mongo raw signal writes, Postgres work-item writes, TimescaleDB metric writes, retry-with-backoff, dead-letter *(verified: 200 signals → 2 work_items, ratio 100×; Redis-restart resilience confirmed)*
- [ ] **Phase 4 — Workflow Engine** (Day 4): State pattern, Strategy pattern (alerters), RCA model + validation, MTTR calculation, transactional state transitions, unit tests
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
| Unit test coverage (core pkgs) | ≥ 60% | — | end Phase 4 |
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
