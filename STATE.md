# STATE.md

> **Purpose:** Live status of the IMS build. Updated at the end of every working session.
> **For AI assistants:** Always read this file first. It tells you where the project is and what to do next. Do not assume; check here.

---

## Current state

**Phase:** 1 (Foundation) — *complete, awaiting review*
**Last session ended:** 2026-05-11. Repo scaffolded. Docker Compose brings Postgres+TimescaleDB, MongoDB, Redis to healthy state. Go module `github.com/kubeboiii/ims` builds; `go run ./cmd/ims` starts an empty Gin server on `:8080` with a `/health` placeholder. Next.js 14 app (App Router, TS, Tailwind) scaffolded in `frontend/` and `pnpm build` is clean. README and decisions log updated.
**Next action:** Begin Phase 2 (Ingestion & Backpressure). First write `docs/phases/phase-2-ingestion.md` (it doesn't exist yet), then implement: `POST /v1/signals` handler, signal model, bounded channel (cap 50K), worker pool (`NumCPU()*2`), per-source token-bucket rate limiter, `/health` upgrade, metrics ticker. Acceptance: vegeta load test sustains 10K rps for 60s with p99 < 50ms.

---

## Phase checklist

Tick boxes as phases complete. Each phase has acceptance criteria in its build-plan section that must be met before ticking.

- [x] **Phase 1 — Foundation** (Day 1): repo scaffolding, Docker Compose with all 4 DBs running, empty Go module, empty Next.js app, README skeleton
- [ ] **Phase 2 — Ingestion & Backpressure** (Day 2): HTTP endpoint, bounded channel, worker pool, token-bucket rate limiter, `/health`, metrics ticker, **load test proves 10K signals/sec**
- [ ] **Phase 3 — Debounce & Persistence Fan-out** (Day 3): Redis Lua debounce, Mongo raw signal writes, Postgres work-item writes, TimescaleDB metric writes, retry-with-backoff, dead-letter
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
| Sustained ingestion | 10,000 sig/sec for 60s | — | end Phase 2 |
| p99 ingestion latency | < 50ms | — | end Phase 2 |
| Debounce reduction ratio | ≥ 60× (100 signals → 1 work item) | — | end Phase 3 |
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
