# Phase 3 — Debounce & Persistence Fan-out: Q&A Study Guide

> **Phase scope:** Make every signal actually do something. Atomic
> Redis Lua debounce, fan out to MongoDB (raw audit), Postgres
> (work_items source of truth), TimescaleDB (signal_metrics). Wrap each
> write in retry-with-backoff; dead-letter on exhaustion. Upgrade
> `/health` to ping every dep.
>
> **Acceptance result:** 200 signals → 2 work_items, 100× reduction.
> Redis-restart resilience verified.

---

## 1. What we built (one paragraph)

Phase 2 left us with a pipeline whose worker just counted and discarded
the signal. Phase 3 plugged in a real **processor** that, per signal:
calls an atomic Redis Lua script to decide "is this a new incident or
part of an ongoing one?", writes the raw signal to Mongo for the audit
log, writes (or updates) a work_item row in Postgres, and records a
metric row in TimescaleDB. Each write retries with exponential backoff;
if it still fails, the payload goes to a "dead_letter" collection in
Mongo for a human to look at later. `/health` now pings each backend
with a 500ms timeout so operators see *which* dependency is breaking.

---

## 2. The fundamentals

### Q: What's "polyglot persistence"?
**A:** Using *different* databases for different data shapes / access
patterns in one system. Our four are:

| Store | What it holds | Why this one |
|---|---|---|
| Postgres | work_items, state_transitions (Phase 4), rca (Phase 4) | ACID transactions, foreign keys, JSON columns. Relational data with strict invariants. |
| Mongo | raw signals (audit), dead_letter | Schemaless documents — every signal's payload is different. High-volume append-only. |
| Redis | debounce window state | In-memory + atomic Lua scripts. Sub-ms reads. The window must expire automatically (10s TTL) — Redis's `EX` flag does this for free. |
| TimescaleDB | signal_metrics (timeseries) | Auto-partitioning by time; queries that filter on `ts` skip entire chunks. Lives inside Postgres as an extension, so no extra container. |

**Interview line:** "Each store was picked for the access pattern, not
because it's trendy. Postgres is the source of truth; the others are
derivatives — eventually consistent on purpose."

### Q: What does "atomic check-then-act" mean, and why is it hard?
**A:** Many operations are read-decide-write: "is there an active
debounce window? if no → create one; if yes → increment its count."
Between the read and the write, *another goroutine on another worker
on another machine* could do the same read, see the same answer, and
both write. Now you have two work_items where you wanted one.

Two ways to fix:
1. **Lock everything** — pessimistic; one writer at a time. Slow.
2. **Atomic in the storage layer** — let the DB do the whole read+write
   as one indivisible operation. Redis Lua scripts run *single-threaded
   on the Redis server*, so the script body IS atomic across all
   clients. That's our pick.

### Q: What is a Redis Lua script and why does it solve the race?
**A:** Redis lets you submit a small Lua program with `EVAL` (or `EVALSHA`
if you've cached it via `SCRIPT LOAD`). The Redis server runs that
program **to completion** before serving any other request. So our
"check-then-act" — GET window, decide, SET or INCR — happens as one
atomic step. No other client can interleave.

The script lives in
[`backend/internal/debounce/script.lua`](backend/internal/debounce/script.lua)
(CLAUDE.md design rule 4). We embed it in the Go binary via `go:embed`
so we don't depend on a filesystem path at runtime.

### Q: What's a "hypertable" in TimescaleDB?
**A:** A regular Postgres table that auto-partitions itself by a time
column. We declare it with `SELECT create_hypertable('signal_metrics', 'ts')`.
Under the hood Timescale creates child tables ("chunks") covering, say,
one week each. Queries with `WHERE ts > now() - interval '1 hour'` only
touch the most recent chunk; queries on old data don't slow you down.

You write into it like any table (`INSERT INTO signal_metrics ...`);
the partitioning is invisible to your code.

### Q: What's a "connection pool" and why pgx vs database/sql?
**A:** Opening a Postgres TCP connection takes ~10ms (handshake +
authentication). At 10K signals/sec you'd spend most of your time on
handshakes. A pool keeps N connections warm; each query borrows one and
returns it.

`pgx/v5` is the native Postgres driver for Go — faster than stdlib's
`database/sql`/`pq` because it speaks Postgres protocol directly without
the `database/sql` abstraction layer. CLAUDE.md mandates pgx because
this project is supposed to demonstrate "right tool for the job."

### Q: What is "exponential backoff with jitter"?
**A:** When a downstream is failing, you don't want to retry instantly
(it's probably still failing) or wait too long (you'll miss the
recovery). Backoff schedule: 100ms, then 200ms, then 400ms, then
800ms, etc. The "exponential" is the doubling. "Jitter" (random
fuzz on each delay) prevents a stampede where 10,000 workers all
retry at exactly the same moment.

We use `cenkalti/backoff/v4`. Base 100ms × 2, max 3 attempts —
total wall-clock budget of ~700ms before giving up.

### Q: What's a "dead letter"?
**A:** A storage location for items that exhausted their retry
attempts. The producer says "I tried 3 times, I can't process this,
here it is for a human to inspect." Different from "delete and forget"
because the data is preserved.

Our dead letters go to a `dead_letter` Mongo collection. We chose Mongo
because it's the one sink most likely to still be reachable when a
*different* sink failed (e.g., Postgres dead-letter to Postgres
makes no sense).

### Q: What's an "embedded file" in Go (`go:embed`)?
**A:** A compile-time directive that embeds a static file's contents
into your binary as a variable:

```go
//go:embed script.lua
var scriptBody string
```

After `go build`, the Lua source is baked into the executable. No
filesystem read at runtime. Useful for SQL templates, HTML, JSON
schemas, etc.

### Q: What's "fan-out"?
**A:** One input → multiple outputs. Per signal we fan out to 3 sinks
(Mongo, Postgres, Timescale). We do them sequentially in the same
worker goroutine, not in parallel — at 10K signals/sec, spawning 3
goroutines per signal would mean 30K goroutines/sec churning, and the
savings (sequential takes ~3× the latency of parallel) don't justify
the extra GC pressure.

### Q: What is "eventual consistency"?
**A:** Different copies of the same data agree *eventually*, not
immediately. After we write a work_item to Postgres + a metric to
Timescale + the raw signal to Mongo, all three might be at different
"versions" for a few milliseconds. The system is correct (no signal
lost) but momentarily inconsistent.

The opposite is **strong consistency** — every read sees the latest
write. That requires distributed transactions, which we explicitly
don't do.

---

## 3. The tech we used

### Q: What's `golang-migrate`?
**A:** A CLI tool that runs SQL files in numeric order against a DB.
Files are named `001_name.up.sql` / `001_name.down.sql`. The tool
keeps a `schema_migrations` table tracking which versions have been
applied so re-running is idempotent. `migrate ... up` moves forward,
`migrate ... down` rolls back.

We run it as a separate step (not embedded in the app) so the app
binary doesn't need DDL permissions.

### Q: What's a "context" in `pgx` (and most Go DB libraries)?
**A:** Same `context.Context` you've seen in Phase 2. Every pgx call
takes a `ctx` as first arg. If you `context.WithTimeout(ctx, 500ms)`,
pgx cancels the query after 500ms. That's how `/health` bounds its
per-dep ping budget.

### Q: What's `bson.M` in the mongo driver?
**A:** A map alias: `type M map[string]any`. The most common way to
build ad-hoc documents in mongo-go-driver. `bson.D` is the ordered
variant (a slice of key-value pairs) — used for queries where field
order matters (like index definitions).

### Q: What's `*redis.Script` vs `client.EvalSha(...)`?
**A:** Both run a Lua script. `EvalSha` directly takes a SHA; if Redis
has evicted the script cache (e.g., restart), you get `NOSCRIPT` and
your code breaks. `*redis.Script.Run` wraps that — on `NOSCRIPT` it
auto-uploads the script body via `EVAL` and retries. Same fast path,
but resilient to Redis restarts.

**This was a real bug we fixed.** First implementation used `EvalSha`;
killing and restarting Redis broke debouncing permanently until the
backend was also restarted.

### Q: What's `testcontainers-go`?
**A:** A Go library that spins up Docker containers programmatically
from test code. `tcpostgres.Run(ctx, "timescale/timescaledb:...")`
starts a Postgres container, waits for it to be ready, hands you back
a `ConnectionString()`. `t.Cleanup` tears it down. Tests run against
the real driver against a real DB, not against a mock.

### Q: What's `t.Setenv` and `t.Cleanup`?
**A:** Testing-only helpers from `testing.T`:
- `t.Setenv("KEY", "value")` sets an env var that's automatically
  unset when the test ends. Safer than `os.Setenv`.
- `t.Cleanup(fn)` registers a function to run when the test ends
  (after subtests, in LIFO order). Replaces `defer` for setup/teardown
  in a way that survives table-driven tests.

---

## 4. The design decisions

### Q: Why is Postgres the source of truth and not Mongo?
**A:** Work items have *relations* (RCA, state_transitions, alerts —
Phase 4). They need ACID transactions (a state change MUST also write
the audit row). Postgres has both; Mongo's transactions are clunkier
and don't enforce foreign keys. We bet on Postgres for everything
relational and pushed only the schemaless audit log (raw signals) to
Mongo.

### Q: Why does Redis being down NOT make the system unhealthy?
**A:** Because we have a **graceful fallback**. If the debounce script
fails, we treat every signal as a fresh window → CREATE a new
work_item. The system gets noisier (more work_items than ideal), but
no signal is lost. Once Redis recovers, debouncing resumes on the next
signal — atomically, via `*redis.Script.Run`'s auto-reload.

Contrast with Postgres-down: we have no fallback for work_items (it's
the source of truth). So Postgres-down means writes dead-letter, and
the system enters a degraded state where the operator must manually
recover from `dead_letter`.

That asymmetry is what `criticalDeps` encodes in `/health`. Critical
deps cause 503; non-critical cause 200-degraded.

### Q: Why are we retrying with 3 attempts and not 5 or 1?
**A:** Tradeoff between recovery and latency:
- 1 attempt: any transient blip = dead-letter. Too aggressive.
- 5 attempts: total wall-clock 100+200+400+800+1600 = 3.1s per
  signal. At 10K signals/sec, a partial outage could pile up a backlog
  faster than we drain — workers wedge on retries.
- 3 attempts: 700ms worst-case, catches transient network blips,
  exits fast enough to keep workers responsive.

### Q: Why is `IncrementSignalCount` NOT in a SERIALIZABLE transaction?
**A:** It's a single-row UPDATE. Postgres serializes writes to the
same row at the per-row lock level for free. There's nothing else in
the "transaction" — no other writes that need to happen atomically
with it. Wrapping it in SERIALIZABLE would just add transaction
overhead and increase contention at 10K signals/sec.

State transitions (Phase 4) DO need SERIALIZABLE because they're
multi-statement: SELECT FOR UPDATE → check state → UPDATE → INSERT
audit row. That's where the heavier isolation pays off.

### Q: Why use `GREATEST()` in the `last_signal_ts` update?
**A:** Workers process signals in parallel. Signal A (ts=T+100ms) might
finish processing AFTER signal B (ts=T+50ms) because B's worker was
faster. Without `GREATEST`, `last_signal_ts` would flicker backward.
With it, the column is monotonic.

(This is a real-world subtle bug — parallel processing of timestamped
events. Good interview material.)

### Q: Why is the processor's fan-out sequential, not parallel?
**A:** At 10K signals/sec, parallel fan-out = 30K goroutines/sec
churning. Goroutine creation is cheap (~µs each) but it's not free.
Sequential adds ~3× latency per signal but uses 1/3 the goroutines.
Trade-off favored simplicity + GC pressure. If a sink is consistently
slow, the *pipeline backpressure* (queue fills, 503s) handles it —
not parallel fan-out within one signal.

### Q: Why do we fail-fast at startup if any sink is unreachable?
**A:** Two reasons:
1. **Fast feedback loop**: if you misconfigured `DATABASE_URL`, you
   want to know in 5 seconds, not 30 minutes later when the first
   signal arrives.
2. **The system was designed assuming all sinks are reachable at boot**.
   `/health`'s "degraded" mode is for *runtime* failures (sink went
   down after we'd been running). Booting in a broken state and
   pretending we're "healthy" is dishonest.

The exception: Phase 4+ might want to start before Postgres is fully
ready, with a retry loop. v1 fails fast for clarity.

### Q: Why is the `Pinger` interface defined in `internal/obs/health.go`
and not in the persistence packages?
**A:** Go idiom: **define interfaces where they're consumed, not where
they're implemented.** `obs` is the package that CALLS Ping; the
persistence repos just happen to have a `Ping(ctx)` method that
satisfies the interface. Keeping the interface in `obs` lets `obs`
evolve without forcing every repo to import an "interface package."

### Q: Why does `processor.New` take so many arguments?
**A:** **Constructor injection** (the canonical Go way of doing
"dependency injection"): every dependency is an explicit parameter, so
the constructor signature documents what the processor needs. Tests
inject fakes; production wires real repos. Beats reading from
package-level globals.

The args list is long but readable. A "container" framework (à la
Spring) is over-engineering at this scale.

---

## 5. Tradeoffs

### Q: We're losing transactional consistency across the 4 sinks. Is that OK?
**A:** Yes, deliberately. The alternative — distributed transactions
across heterogeneous stores — needs 2PC (2-phase commit) or sagas
(compensating writes). Both are complex and slow. We instead:
- Treat Postgres as source of truth (the *one* store that must be
  consistent).
- Mongo's audit log is allowed to lag.
- Redis cache is allowed to be briefly stale.
- Timescale metrics are allowed to be eventually consistent.

If any sink loses a write, the dead_letter captures it. A human is the
recovery mechanism.

This is one of the most-asked interview questions. Don't apologize for
it — explain the tradeoff.

### Q: We're not auto-replaying the dead-letter. Why not?
**A:** Most things that hit dead-letter do so because of a *persistent*
problem (schema mismatch, downstream still down, malformed payload).
Auto-replay would loop forever and spike the failure rate. Manual
inspection is the safer policy for v1.

In production you'd want a separate replay tool that runs once a day,
queries dead_letter, and re-attempts each entry — but that's a Phase 7+
feature.

### Q: Why not use a saga/orchestrator framework like Temporal?
**A:** Overkill for a 4-write fan-out. Temporal is fantastic for
long-running workflows with human input (e.g., a multi-day order
fulfillment). Per-signal fan-out is sub-millisecond; the orchestration
"workflow" fits in 30 lines of Go. Adding Temporal would mean another
container, another DB, another set of operational concerns.

### Q: Why do we keep the rate limiter per-process instead of in Redis?
**A:** Per-process is fast (lock-free, ~50ns). Redis-backed would
survive multi-replica deploys, but at the cost of one Redis round-trip
per request (~1ms). We deploy one replica in v1; we can lift this to
Redis when we scale out.

### Q: Why is the Lua script body so short?
**A:** Two reasons:
1. **Atomicity has a cost.** While the script runs, Redis serves no
   other requests. A 1ms script holds up 1ms of all other Redis traffic.
   Keep it cheap.
2. **Maintainability.** Lua is a small language we don't write much of;
   a 200-line script is hard to review. Ours is 10 lines.

---

## 6. Interview gotchas

> *Tight answers; expand from above if pressed.*

**1. "Walk me through what happens to a signal after a worker picks it up."**
1. Worker calls `processor.Process(ctx, signal)`.
2. Processor calls `debouncer.Process(ctx, signal.ComponentID)` →
   `*redis.Script.Run` → atomic Lua check-then-act → returns
   `(work_item_id, action, count)`.
3. Processor calls `signals.Insert(ctx, signal, workItemID)` →
   `mongo.InsertOne`. Wrapped in backoff. Dead-letter on exhaustion.
4. If action=CREATED: `workItems.Insert(ctx, NewWorkItem(...))` →
   pgx INSERT. If action=JOINED: `workItems.IncrementSignalCount(...)`
   → pgx UPDATE with `GREATEST(last_signal_ts, $2)`.
5. `metrics.Insert(ctx, signal, workItemID)` → pgx INSERT into the
   Timescale hypertable.
6. Each sink wrapped in `backoff.Retry` with 3 attempts.
7. Worker returns to channel-wait.

**2. "Why Redis Lua and not application-level locking?"**
- Application locks (e.g., `sync.Mutex`) only work within one
  process. Multi-replica deploys would race.
- Distributed locks (Redlock, etc.) add latency and have edge-case
  bugs (clock skew).
- Lua scripts atomically execute server-side. Single Redis command
  per signal, no lock acquisition, no deadlock risk. Best fit.

**3. "What happens when Redis restarts?"**
The script cache is wiped. The first EVALSHA after restart returns
`NOSCRIPT`. `*redis.Script.Run` catches that, re-uploads the script
body via EVAL, and retries the call. One extra round-trip on the
first call; everything fast after that. We verified this empirically:
kill Redis → fallback CREATE mode → restart Redis → next call
debounces correctly.

**4. "Why is the work_items count the sum of signal_counts, not a
COUNT(*) of mongo signals?"**
Two different things:
- `work_items.signal_count` = how many signals were attached to THIS
  work_item. Tracked in Postgres for the detail view.
- `mongo.signals` count by component = all raw signals, regardless of
  debounce. The audit log.

They sum to the same number for one component (200 = 100 + 100 across
two work items in our acceptance demo).

**5. "How do you handle a signal whose work_item_id no longer exists in
Postgres?"**
`IncrementSignalCount` returns `ErrNotFound` if 0 rows were affected
(e.g., the row was deleted, or — more realistic — the debounce cache
in Redis points at a stale UUID because Postgres lost the row and
Redis didn't know). The processor logs the error and dead-letters the
signal. In practice this is extremely rare; v1 doesn't auto-recover.

**6. "What's the latency of a single signal end-to-end?"**
Under nominal load (the 10K/sec demo):
- Ingest handler: ~1ms (Phase 2 measurement).
- Worker pick-up: ~µs (channel receive).
- Debounce Redis call: ~0.5ms (single Lua call).
- Mongo insert: ~1ms.
- Postgres insert: ~1ms.
- Timescale insert: ~1ms.

Total: ~5ms per signal at the processor level. p99 ingest stays
sub-2ms because the handler returns 202 before any of this runs.

**7. "What does `*redis.Script.Run`'s first call look like?"**
- First call: client sends `EVALSHA <sha> <keys> <args>`. If Redis has
  the script cached, executes immediately.
- If not cached (NOSCRIPT error): client sends `EVAL <body> <keys>
  <args>`. Redis runs it AND caches the SHA. Subsequent calls hit
  the fast EVALSHA path.

So the steady state is one round-trip per call.

**8. "Why backoff with `WithMaxRetries(2)` and not `WithMaxRetries(3)`?"**
Because `backoff.WithMaxRetries(N)` counts **retries**, not total
attempts. N=2 means initial try + 2 retries = 3 total attempts. We
expose `MaxAttempts=3` in our public API and translate internally to
`MaxRetries=N-1`.

**9. "Walk me through the Postgres state of a work_item after 200
signals to the same component."**
The Lua script counts signals up to `MaxSignals=100`. Signal 101
opens a *new* work_item. So 200 signals → 2 work_items, each with
`signal_count=100`. The acceptance demo confirms exactly this
behaviour.

**10. "How would you scale Mongo writes if 10K/sec wasn't enough?"**
Options in order of effort:
- **Batch writes**: collect 100 signals in a worker, `InsertMany` once.
  ~50× fewer round-trips. Cost: lose the "every signal is immediately
  in Mongo" guarantee.
- **Shard the collection** on `component_id`. Mongo handles this
  natively; routes writes by hash of the shard key.
- **Replicate workers**: deploy more backend instances. They all share
  the Redis debounce state via the Lua script, so debouncing still works.

**11. "Why is `IncidentStart` in the work_item nullable?"**
Phase 3 only creates work_items in OPEN state. Phase 4's RCA flow
populates `IncidentStart` (defaults to `first_signal_ts`, editable by
the RCA form). Nullable = "set in Phase 4." Using a pointer type
(`*time.Time`) distinguishes "not set yet" from "epoch zero."

---

## 7. Things you should be able to do after Phase 3

- [ ] Run the acceptance demo (`./scripts/simulate-component-storm.sh`)
      and explain every number in the output.
- [ ] Tell me which sink is the source of truth and which two are
      derivatives, with one sentence justifying the choice.
- [ ] Walk through the Lua script line-by-line and explain why it must
      run server-side, not client-side.
- [ ] Predict what `signal_count` and `last_signal_ts` will be after
      firing 150 signals at one component over 5 seconds (one work_item
      with `signal_count=100`, plus a new one with `signal_count=50`).
- [ ] Kill the Redis container while the backend is running; explain
      the `/health` output and why work_items are still being created.
- [ ] Kill the Postgres container; check the `dead_letter` collection
      to confirm work_item writes are landing there.
- [ ] Re-run `migrate down --all` then `migrate up` and confirm tables
      are recreated cleanly.
- [ ] Modify the debounce window via env var (`VELLUM_DEBOUNCE_WINDOW_SECONDS=30`),
      restart the backend, and verify the new window length in Redis
      (TTL is 30s on `debounce:CACHE_01:work_item`).
- [ ] Add one new field to the Signal model and trace it through:
      where does Validate() check it? where does Mongo store it? does
      Postgres need a migration?

If all of those feel comfortable, Phase 3 is solid in your head.
