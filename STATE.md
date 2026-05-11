# STATE.md

> **Purpose:** Live status of the IMS build. Updated at the end of every working session.
> **For AI assistants:** Always read this file first. It tells you where the project is and what to do next. Do not assume; check here.

---

## Current state

**Phase:** 5 (gRPC + Frontend) — *complete, on branch `phase-5-grpc-frontend`*
**Last session ended:** 2026-05-11. Defined `backend/proto/ims/v1/signals.proto` (bidi `SignalService.IngestSignals`), wired buf 1.69 with local protoc-gen-go/grpc plugins, generated and checked in `signals.pb.go` + `signals_grpc.pb.go`. Implemented `internal/ingest/grpc.go` (bidi-stream server, shares the pipeline with HTTP per FR-1.3) with 4 bufconn-backed unit tests. Wired the gRPC server into `cmd/ims/main.go` on `:9090` alongside HTTP, with graceful shutdown of both. Added inline CORS middleware (no extra dep) for the dashboard. Added `GET /v1/incidents/:id/signals` HTTP endpoint backed by `mongo.SignalRepository.ListByWorkItem` (paginated, UUIDs returned as strings not BSON Binary). Mapped pgx SQLSTATE 40001 to `pg.ErrSerializationFailure → 409 Conflict` in the API error helper so concurrent-update races during the workflow tx return a retryable code instead of a 500. **Frontend rebuilt to THEME.md spec**: matte-black 5-level surface palette, lime brand accent, JetBrains Mono + Inter via next/font, severity-keyed colors (P0=red, P1=orange, P2=amber, P3=blue), 8 hand-rolled components (Nav, SeverityBadge, StatePill, Button, StatCard, SignalRateChart, IncidentRow, PayloadJSON) mapped through Tailwind config tokens. Added recharts, framer-motion, @tabler/icons-react. All 3 pages re-laid-out per THEME.md §7 (live feed = hero chart + 4 stat cards + filter bar + incident table; detail = 2-column 3/5+2/5; RCA = 720px column with dark form + lime primary). P0+OPEN dot pulses (§6.7). JSON payload syntax-highlighted (keys=secondary, strings=emerald, numbers=amber, booleans=blue). `prefers-reduced-motion` honored globally. `pnpm build` clean across 4 routes; all pages serve HTTP 200 with no frontend errors. End-to-end smoke: 10 signals streamed via gRPC → work_item created → PATCH OPEN→INVESTIGATING→RESOLVED via REST → POST RCA returned 201 with MTTR populated, PagerDuty stub fired for the gRPC-originated P0. `go test -race ./...` clean across 13 packages.
**Next action:** Begin Phase 6 (Resilience & Simulation). First write `docs/phases/phase-6-resilience.md`. Then: build the failure simulator (`scripts/simulate-outage.go` per CLAUDE.md) — cascading RDBMS → cache → MCP scenario that proves the system degrades + recovers correctly. Add an integration test (everything wired up, ephemeral containers, signal-through-RCA flow). Add a concurrency stress test that hammers the workflow engine to surface any remaining 40001 races. Bug-fixing pass.

---

## Phase checklist

Tick boxes as phases complete. Each phase has acceptance criteria in its build-plan section that must be met before ticking.

- [x] **Phase 1 — Foundation** (Day 1): repo scaffolding, Docker Compose with all 4 DBs running, empty Go module, empty Next.js app, README skeleton
- [x] **Phase 2 — Ingestion & Backpressure** (Day 2): HTTP endpoint, bounded channel, worker pool, token-bucket rate limiter, `/health`, metrics ticker, **load test proves 10K signals/sec** *(verified: 600K req over 60s, 100% success, p99 1.89 ms)*
- [x] **Phase 3 — Debounce & Persistence Fan-out** (Day 3): Redis Lua debounce, Mongo raw signal writes, Postgres work-item writes, TimescaleDB metric writes, retry-with-backoff, dead-letter *(verified: 200 signals → 2 work_items, ratio 100×; Redis-restart resilience confirmed)*
- [x] **Phase 4 — Workflow Engine** (Day 4): State pattern, Strategy pattern (alerters), RCA model + validation, MTTR calculation, transactional state transitions, unit tests *(verified: PRD G3 end-to-end via curl + concurrent-close test passes)*
- [x] **Phase 5 — gRPC + Frontend** (Day 5): gRPC streaming endpoint sharing pipeline, Next.js dashboard (live feed, detail, RCA form) *(verified: 10 signals via gRPC → PRD G3 closure via dashboard endpoints)*
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
