# Prompts — Per-Phase Narrative

> **Purpose:** Honest account of how this project was built with Claude. One section per phase. Each section captures: the goal, the actual prompt approach, what came out clean vs. what I had to push back on, and what I'd do differently.

This file is not a prompt-by-prompt dump. It's a working narrative — the kind a reviewer reads to understand the *process* alongside the artifact.

---

## Phase 1 — Foundation

### Goal
Repo scaffolding, Docker Compose with all 4 DBs running, empty Go module, empty Next.js app, README skeleton.

### Approach
Wrote `docs/00-master-prd.md` and `docs/01-architecture.md` first (no code yet), then asked Claude to scaffold the directory layout per `CLAUDE.md`. Single-file generations: `docker/compose.yaml`, `backend/cmd/vellum/main.go`, `backend/migrations/001_init.sql`, `frontend/package.json`. The phase file (`docs/phases/phase-1-foundation.md`) anchored the acceptance criteria.

### What worked first try
- Docker Compose with named volumes and healthchecks
- Go module structure (cmd/ + internal/)
- Tailwind + Next.js 14 App Router scaffolding

### What I had to push back on
- First scaffold used `database/sql` for Postgres — I had to explicitly redirect to pgx v5 (the stack table in `CLAUDE.md`). Lesson: re-read the stack table at the start of every prompt.
- Initial migrations missed `created_at`/`updated_at` columns — convention from `CLAUDE.md` §SQL was buried; pulled it up.

### What I'd do differently
Start with `CLAUDE.md` as the first file written. Every subsequent phase prompt then says "honour CLAUDE.md" and the model behaves better.

---

## Phase 2 — Ingestion & Backpressure

### Goal
HTTP `POST /v1/signals` endpoint, bounded channel, worker pool, token-bucket rate limiter, `/health`, metrics ticker. Load test at 10K signals/sec, p99 < 50ms.

### Approach
Prompt was structured around four critical design rules from `CLAUDE.md`:
1. Ingestion never blocks on persistence (non-blocking channel send).
2. Backpressure returns 503 within 5ms.
3. Rate limiter is per-source.
4. Shutdown drains the queue with a 30s deadline.

Each rule got its own test before any handler code was written.

### What worked first try
- `internal/pipeline/pipeline.go` — bounded `chan Signal`, worker pool with `errgroup`, graceful shutdown via `context.WithTimeout`. Clean idiomatic Go.
- `vegeta` load-test script (`scripts/load-test.sh`) with REPORT_DIR for repeatable results.

### What I had to push back on
- First version of the rate limiter used a global mutex on a map — I asked for `golang.org/x/time/rate`'s per-source `Limiter` instead, keyed by source IP and garbage-collected.
- The bounded channel was initially `cap = 1000`. PRD targets 10K/s, so a 100ms drain at full capacity → 1000 items is tight. Bumped to 50,000 (5s of headroom at peak rate).

### Acceptance result
600,000 requests over 60 seconds. 100% success rate. p99 latency 1.89ms. Logged in `decisions.md`.

### What I'd do differently
Write the load-test script in Day 1 alongside the scaffolding, not Day 2 at the end. Having `./scripts/load-test.sh` available made every subsequent phase faster to validate.

---

## Phase 3 — Debounce & Persistence Fan-out

### Goal
Redis Lua atomic debounce, Mongo raw signal writes, Postgres work-item writes, TimescaleDB metric writes, retry-with-backoff, dead-letter.

### Approach
The debouncer is the most-likely-to-be-discussed piece in interviews, so the prompt explicitly asked for:
- The Lua script in its own file (`backend/internal/debounce/script.lua`) — not a Go string literal.
- The script loaded at startup via `SCRIPT LOAD`, called via EVALSHA.
- A graceful-degradation path when Redis is down (FR-3.6 — fall through to always-CREATED).

### What worked first try
- The Lua script. Wrote it in 30 lines: GET window, check count, INCR or CREATE+SET, return action+id+count.
- Mongo writer with bulk inserts (`BulkWrite`) for throughput.

### What I had to push back on
- First version persisted to Postgres synchronously on the hot path. Violates design rule #1. Refactored into a `processor` goroutine that reads from the same bounded channel.
- The retry-with-backoff was open-loop (no max attempts). Added exponential backoff with 3 attempts then dead-letter.

### Acceptance result
200 signals to one component → 2 work_items (debounce ratio 100×). Killed Redis mid-run; signals kept flowing (degraded mode logged once). Restarted Redis; ratio returned to 100×. Logged.

### What I'd do differently
Add a `TestProcess_ConcurrentSameComponent` test on day 1 of this phase, not at the end. The atomicity property is the hardest part of the phase to get right; testing it first would have caught one bug I found by hand.

---

## Phase 4 — Workflow Engine

### Goal
State pattern for Work Item lifecycle, Strategy pattern for alerters, RCA model + validation, MTTR calculation, transactional state transitions, unit tests.

### Approach
This phase is where the "LLD" rubric category is earned, so the prompt emphasized:
- State pattern: one struct per state, each implementing a single `CanTransitionTo(target, ctx) error` method. Transitions live in those methods. No switch statements anywhere.
- Strategy pattern: alerters as a registry keyed by `(component_type, severity)`. Adding one is a one-file change.
- RCA gate: `ResolvedState.CanTransitionTo(ClosedState)` is the only place RCA is checked. Single source of truth.

### What worked first try
- State pattern — clean separation, easy to read.
- RCA validation as a `Validate() []FieldError` method on the model, returning a structured error array. The frontend later consumed this exact shape.

### What I had to push back on
- First Postgres transaction used `READ COMMITTED`. I asked for `SERIALIZABLE` because two writers updating the same work_item could split-brain a transition. Mapped pgx SQLSTATE 40001 to a retryable 409 Conflict.
- The first MTTR computation used `time.Since(work_item.created_at)`. Wrong — MTTR is `incident_end - incident_start`, both editable in the RCA. Fixed.

### Acceptance result
End-to-end via curl: POST signal → PATCH state OPEN→INVESTIGATING→RESOLVED → POST RCA → status=CLOSED, MTTR computed. RCA gate proven: 422 with field-level errors when RCA missing or incomplete. Concurrent-close test passes.

### What I'd do differently
The `alert/strategy.go` registry started as a `map[string]Alerter` keyed by `severity`. It should have been a two-key map from the start (component_type + severity). Refactored once.

---

## Phase 5 — gRPC + Frontend

### Goal
gRPC server-streaming endpoint sharing the HTTP ingestion pipeline. Next.js dashboard with live feed, incident detail, RCA form.

### Approach
This was the biggest phase by volume. Split into three sub-prompts:
1. **gRPC plumbing**: `signals.proto` → `buf generate` → `internal/ingest/grpc.go` sharing the channel with the HTTP handler.
2. **Dashboard MVP**: 3 pages per PRD §7. Polling at 2s, no WebSocket.
3. **Landing page + demo pages**: a `LANDING.md` spec I wrote first, then 10 sections built piece by piece.

### What worked first try
- gRPC bidi-stream handler. Same `Pipeline.Enqueue()` call as HTTP. The ACK-stream pattern fit naturally.
- The `STATE` pattern between SeverityBadge / StatePill / `THEME.md` made the dashboard surprisingly easy to extend.
- Shiki server-side syntax highlighting for the code-tabs section on the landing page (zero bundle cost for the user).

### What I had to push back on, a lot
- The first dashboard was a wall of recharts charts. Cut three. Added a stacked severity bar (more useful than four StatCards at a glance).
- buf's remote plugins were down for ~30 minutes on the day I tried them. Switched to local plugins (`protoc-gen-go`, `protoc-gen-go-grpc`) in `buf.gen.yaml`. Logged in decisions.md.
- The Annotation component (hand-drawn arrow + violet label) drifted across viewports. Tried three times to fix the anchoring. Eventually removed both annotations entirely. Logged.
- Dev-server CSS chunks went stale mid-session (the dreaded 404 on `/_next/static/css/app/layout.css`). Solution: use `pnpm build && pnpm start` for any screenshot or demo session. Cached `pnpm dev` is fine for live editing.

### Acceptance result
Full lifecycle verified end-to-end: gRPC ingest → debounce → dashboard live feed updates within 2s → click into incident → advance state → submit RCA → MTTR shown.

### What I'd do differently
Write the `THEME.md` and `LANDING.md` specs *before* committing any frontend code. I wrote them after the first dashboard attempt and had to throw out about 40% of what I'd already shipped.

---

## Phase 6 — Resilience & Simulation

### Goal
Failure simulator script, integration test, concurrency stress test, bug fixes.

### Approach
Three deliverables, smallest-scope-first to maximize feedback velocity:
1. Concurrency stress test (in the existing test file) — fastest signal, highest value.
2. E2E integration test against the live backend (build-tagged so unit tests stay fast).
3. Headless `scripts/simulate-outage.go` matching the three scenarios from the `/simulate` UI page.

### What worked first try
- The stress test. `300 goroutines × same component_id @ cap=100 → expect exactly 3 distinct work_items`. The atomicity property held under `-race`.
- The simulator's debounce-ratio math: snapshot `/v1/incidents` before, run the burst, snapshot after, compute `accepted/(after-before)`. Cache scenario hit exactly 100× compression on the first run.

### What I had to push back on
- The integration test initially tried to spin up testcontainers for all four DBs. That added 20+ seconds to the run. Switched to "assume the dev compose stack is up; point at :8080". Build tag `integration` keeps it out of the default test run.

### Acceptance result
- Both stress tests pass under `-race`.
- E2E integration test passes in 0.29s.
- All three simulator scenarios match their predicted work_item counts exactly. Cache = 100× compression.

### What I'd do differently
Nothing major. This phase was the cleanest of the seven.

---

## Phase 7 — Documentation & Polish

### Goal
Final README, Mermaid architecture diagrams, `prompts.md` (this file), dry-run from fresh clone, submission packaging.

### Approach
Did the audit first (what docs exist, what's missing, what's stale) before writing any new prose. Asked Claude for an inventory of every doc surface. Then made one pass through:
1. Replace ASCII diagrams with Mermaid (renders on GitHub, diff-able in git, no image files to keep in sync).
2. Promote the project's elevator pitch to the README top, with a Mermaid diagram showing the pipeline.
3. Append Phase 6 + Phase 7 acceptance sections to README.
4. Update STATE.md.
5. Write this file.

### What I'd do differently
Write `prompts.md` incrementally — one section per phase as that phase closes. Doing it all at the end means relying on memory.

---

## Meta: how I worked with Claude across all 7 phases

1. **`CLAUDE.md` is the operating contract.** Every session referenced it. The tech-stack table, the four design rules, the "ask before doing X" list — all there.
2. **`STATE.md` is the resume token.** Every session I read it first, every session I updated it at the end. Without it, Claude restarts a project from scratch each day.
3. **Phase files are the acceptance test.** Each phase had its own `docs/phases/phase-N-*.md` with explicit "done means…" criteria. I refused to merge until every box was ticked.
4. **`decisions.md` is the rationale graveyard.** 27 entries across the project. Every deviation, every fork-in-the-road, recorded with date + context + alternatives considered.
5. **Push back on the first attempt 30% of the time.** Not because Claude was wrong, but because the first draft optimized for "looks reasonable" over "honours the four design rules". Re-reading the prompt under the rules usually fixed it.
6. **The frontend is where AI assistance accelerated me the most.** 10 landing sections + a full dashboard + 3 demo pages in roughly 6 hours of attention. Without an assistant: easily 3 days.
7. **The backend is where AI assistance saved the least time.** Idiomatic Go with strict design rules is something a human writes faster than they can explain to a model. The wins were elsewhere: tests, scripts, the README, this file.

If you're reviewing this project: **read [`docs/decisions.md`](decisions.md) before reading the code**. Every counter-intuitive choice is justified there.
