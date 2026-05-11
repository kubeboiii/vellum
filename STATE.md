# STATE.md

> **Purpose:** Live status of the IMS build. Updated at the end of every working session.
> **For AI assistants:** Always read this file first. It tells you where the project is and what to do next. Do not assume; check here.

---

## Current state

**Phase:** 7 (Documentation & Polish) — *in progress on branch `phase-7-docs-polish`*. All 7 phases complete; final docs pass in flight.
**Last session ended:** 2026-05-11. Merged PRs #6 (Phase 5 dashboard + landing + demo pages) and #7 (Phase 6 resilience + simulation). Phase 6 shipped: concurrency stress test (300 goroutines × same component_id @ cap=100 → exactly 3 work_items, passes under `-race`), E2E integration test against live backend on `:8080` (0.29s, behind `integration` build tag), and headless `scripts/simulate-outage.go` CLI with three scenarios. Cache scenario hits 100× debounce compression; aggregate across all three scenarios = 51× (RDBMS + MCP intentionally fan across multiple components). Phase 7 in flight: replaced ASCII diagrams in `docs/01-architecture.md` §2/§3 with Mermaid; added a Mermaid pipeline diagram + elevator pitch to README top; added Phase 6 + Phase 7 acceptance sections + a "How to demo" runbook. Remaining: `docs/prompts.md`, project-notes index + Phase 6 Q&A, `.env.example` audit, fresh-clone dry-run, decisions.md Phase 7 entries, commit + PR.
**Next action:** Continue Phase 7 finishing steps in order: prompts.md → project-notes/phase-6 + 7 Q&A → .env.example audit → dry-run → decisions.md → commit + PR #8.

---

## Phase checklist

Tick boxes as phases complete. Each phase has acceptance criteria in its build-plan section that must be met before ticking.

- [x] **Phase 1 — Foundation** (Day 1): repo scaffolding, Docker Compose with all 4 DBs running, empty Go module, empty Next.js app, README skeleton
- [x] **Phase 2 — Ingestion & Backpressure** (Day 2): HTTP endpoint, bounded channel, worker pool, token-bucket rate limiter, `/health`, metrics ticker, **load test proves 10K signals/sec** *(verified: 600K req over 60s, 100% success, p99 1.89 ms)*
- [x] **Phase 3 — Debounce & Persistence Fan-out** (Day 3): Redis Lua debounce, Mongo raw signal writes, Postgres work-item writes, TimescaleDB metric writes, retry-with-backoff, dead-letter *(verified: 200 signals → 2 work_items, ratio 100×; Redis-restart resilience confirmed)*
- [x] **Phase 4 — Workflow Engine** (Day 4): State pattern, Strategy pattern (alerters), RCA model + validation, MTTR calculation, transactional state transitions, unit tests *(verified: PRD G3 end-to-end via curl + concurrent-close test passes)*
- [x] **Phase 5 — gRPC + Frontend** (Day 5): gRPC streaming endpoint sharing pipeline, Next.js dashboard (live feed, detail, RCA form), full landing page, demo pages *(verified: PRD G3 closure via dashboard endpoints; merged in PR #6)*
- [x] **Phase 6 — Resilience & Simulation** (Day 6): concurrency stress test, E2E integration test, `scripts/simulate-outage.go` *(verified: cache scenario 100× compression, ratio matches `ceil(N/100)` math exactly; merged in PR #7)*
- [ ] **Phase 7 — Documentation & Polish** (Day 7): Mermaid diagrams, README final pass, `prompts.md`, dry-run, submission packaging *(in progress on branch `phase-7-docs-polish`)*

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
