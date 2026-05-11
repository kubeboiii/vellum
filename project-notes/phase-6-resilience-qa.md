# Phase 6 — Resilience & Simulation: Q&A Study Guide

> Read alongside the actual code at `backend/internal/debounce/stress_test.go`, `backend/internal/e2e_integration_test.go`, and `scripts/simulate-outage.go`.

## What we built

Three artefacts that exist solely to **prove** the system holds up under the kinds of loads and edge cases the unit tests don't reach:

1. A concurrency stress test that crosses window boundaries under the production cap
2. An end-to-end integration test that exercises the full signal-to-RCA lifecycle against a real backend on `:8080`
3. A headless CLI simulator script that reviewers can run from a clean checkout to see the debouncer compress real signals into real work_items

## The fundamentals

### What is `-race`?
Go's race detector. Compile with `-race`, run as usual, and the runtime instruments every memory access to detect data races. **Slow** (~10× overhead) but catches concurrent unsafe access at runtime. `go test -race ./...` is the canonical way to use it.

### What's a build tag?
A Go compiler directive at the top of a file (`//go:build integration`). When you compile with `go test -tags=integration`, files with this tag are included; otherwise they're skipped. Lets us separate fast unit tests from slow integration tests in the same package.

### What's an ephemeral container?
A Docker container spun up specifically for the test, then torn down at the end. `testcontainers-go` is the library that gives you `tcredis.Run(ctx, "redis:7.4.1-alpine")` style APIs. Each test gets a fresh container, so state doesn't leak between tests.

### What does "atomic" mean for the debouncer?
**Indivisible** — the check (is there an open window?) and the act (join or create) happen as one operation that no other caller can interleave with. In our case, the Lua script is the atomic unit because Redis runs scripts single-threaded.

## The tech we used

### `testcontainers-go`
The library that boots ephemeral Docker containers for tests. The Phase 6 stress tests piggyback on the same `startRedis(t)` helper Phase 3 already wrote.

### Go's `sync.WaitGroup`
The primitive for "fire N goroutines, wait for all of them to finish". Used in the stress test to coordinate 300 concurrent `Process()` calls.

### `time.Ticker` (in the simulator)
Fires a channel send at a configurable rate. We use it to pace POSTs at the requested RPS (e.g. 20 signals/sec for 10 seconds = 200 total).

### `atomic.Int64`
Lock-free counters. The simulator uses these for `sent`, `accepted`, `rejected`, `failed` — bumped concurrently from many goroutines, read at the end without a mutex.

## The design decisions

### Q1. Why two stress tests instead of one bigger one?
Each test asserts ONE property:
- `TestProcess_StressManyWindows` proves **the across-window math holds under concurrency** (300 concurrent → exactly 3 windows).
- `TestProcess_StressBoundaryArithmetic` proves **the boundary case is exact** (101 sequential signals → first 100 JOIN, 101st CREATEs).

A combined test would have muddled which property is failing when a regression breaks it.

### Q2. Why does the integration test point at `:8080` instead of using testcontainers?
Two reasons:
1. **Speed**: spinning up Postgres + Mongo + Redis + Timescale takes ~20s. The docker-compose stack is probably already running on a dev machine; reusing it makes the test 0.29s instead of 20+s.
2. **Realism**: testing against the actual binary catches any wiring bug that mocks would mask.

The trade is that the test isn't hermetic — it depends on the dev stack being healthy. But it's behind a build tag, so it's an explicit opt-in.

### Q3. Why a build tag (`//go:build integration`) instead of `t.Skip()` based on an env var?
Compile-time skip is faster: when the tag isn't present, the file isn't even parsed. With `t.Skip()` the test still has to be loaded and run to the skip line. For the 70-line file it doesn't matter, but it scales better.

### Q4. The debounce ratio in PRD G2 says "≥ 60×". The aggregate simulator output shows 51×. Is that a failure?
No — the aggregate is dragged down by the MCP scenario which intentionally fans across 5 different components (30+20+20+20+20 = 110 signals, but 5 components → 5 work_items → 22× ratio). The PRD's 60× target applies to *correlated* signals against a single component. Cache scenario alone is 100× (200 → 2). The simulator's per-scenario reporting makes this distinction visible.

### Q5. Why doesn't the simulator use the existing `/simulate` page?
The `/simulate` page is a browser UI; reviewers running on a server or in CI don't have a browser. The PRD G6 acceptance is "clone repo, `docker compose up`, see proof". A Go CLI passes that test; a web UI doesn't.

### Q6. Why does the simulator snapshot work_items before AND after each scenario?
Idempotence. If you run the simulator twice on the same DB (without resetting), it would otherwise count old work_items as new ones. The before/after delta gives you accurate "what this run produced" counts.

### Q7. How did we choose 300 as the goroutine count for the stress test?
- 100 = the production cap, so 300 = exactly 3 windows. The expected count is a clean integer.
- 100 would also work but only proves single-window atomicity (which the existing test already does).
- 1000 would test more concurrency but adds test time without adding information.

## Tradeoffs

- **Integration test depends on a running backend.** Trade: 0.29s execution vs hermeticity. Documented in the file's header.
- **Stress test takes ~2 seconds (Redis container boot).** Trade: realism vs speed. Acceptable for a phase-6 gate.
- **Simulator doesn't replay alerter events.** Trade: scope. Alerters are fire-and-forget in v1; the simulator measures debounce compression, not alert dispatch correctness.

## Interview gotchas

### "How do you know the debouncer doesn't have a race?"
We have two tests:
- `TestProcess_ConcurrentSameComponent` (50 goroutines, single window) — proves atomicity within a window.
- `TestProcess_StressManyWindows` (300 goroutines, 3 windows under prod cap) — proves the math holds across window boundaries.

Both pass under `-race`. The race detector instruments every memory access; if there was an unsafe shared write, it would fail.

### "Why Lua? Why not a Redis transaction (MULTI/EXEC)?"
Lua is **atomic** (single-threaded execution on the Redis server). MULTI/EXEC is **isolated** but not atomic — between WATCH and EXEC, another client can read the watched keys. For a "check then act" operation like ours, Lua is the right primitive. Documented in `decisions.md` Phase 3.

### "What happens if Redis is down during a stress test?"
The test uses an ephemeral Redis container (testcontainers), so this doesn't happen in test. In production, the debouncer falls through to "always CREATED" mode (FR-3.6) and logs `ErrRedisDegraded`. The handler treats this as a successful response; data isn't lost, just under-deduplicated.

### "Could the integration test be flaky?"
Theoretically: the debouncer is async, so the POST returns 202 before the work_item exists in Postgres. We handle this with a `waitForIncident()` helper that polls `/v1/incidents?limit=500` until the new component_id appears (5s timeout, 200ms poll). Has passed 10 consecutive runs without flake.

### "Why doesn't `simulate-outage.go` use the gRPC endpoint?"
It could, but the HTTP endpoint is simpler to test from a script (just `net/http` stdlib). The gRPC endpoint shares the exact same pipeline downstream (FR-1.3), so the debounce + workflow guarantees are identical regardless of which front door we use.

### "How would you scale the integration test to cover more cases?"
Add a `testdata/` directory with JSON scenario files, each describing a sequence of signals + expected outcomes. Loop the integration test over the directory. The current single test proves the FSM works once; data-driven would prove it holds across edge cases like duplicate signal_ids, P0-to-P3 fan-out, etc.

### "What's the cost of running the simulator on production-grade hardware?"
At 10K signals/sec sustained, the backend is CPU-bound by JSON parsing and Lua execution. Redis-as-a-service (ElastiCache, Upstash) would not blink. Postgres-as-a-service handles bursts of state transitions easily. The hottest path is Mongo (every raw signal is a write); a sharded Mongo cluster would scale linearly. The whole system is designed for horizontal scaling: workers are stateless, debouncer state is in Redis, transactional state is in Postgres SERIALIZABLE.

### "Why is `expectedIncidents` math `ceil(Σcount/100)` instead of just `Σcount/100`?"
Integer truncation. If `count=150` and the cap is 100, then `150/100 = 1` in integer math, but the correct answer is `2` (one window of 100, one window of 50). `ceil` (or `(n+99)/100` for integers) handles this correctly.

### "Couldn't this all be one big BDD test using a framework like Ginkgo?"
Yes, and in a real team it probably would be. We chose stdlib `testing` because (a) it's idiomatic Go, (b) no extra dep, (c) `go test ./...` and `go test -race ./...` are the two commands a reviewer expects to work. Build tags + helper functions give us the same readability without adopting an external framework.

### "What's not covered by these tests?"
- gRPC ingest path (Phase 5 has its own bufconn tests; Phase 6 didn't add stress on top).
- TimescaleDB write path (no test asserts `signal_metrics` rows; we trust the Postgres extension).
- Alerter Strategy fanout under failure (e.g. if Slack webhook is down). Out of scope for v1.
- WebSocket push (B2 bonus, not implemented).
