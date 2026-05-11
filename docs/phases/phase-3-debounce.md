# Phase 3 — Debounce & Persistence Fan-out

> **Day:** 3 of 7
> **Depends on:** Phase 2 (Ingestion & Backpressure) merged on main.
> **References:** `00-master-prd.md` §4.3, §4.4, §4.8, §5.2; `01-architecture.md` §3.2, §5, §6, §9.
> **Out of scope:** Workflow / state machine (Phase 4), gRPC ingestion (Phase 5), failure simulator (Phase 6).

---

## 1. Goal in one sentence

Swap Phase 2's `NoopProcessor` for the real one: every accepted signal
runs through an **atomic Redis Lua debounce** to decide "join existing
Work Item or create new one," then fans out to **MongoDB** (raw audit),
**Postgres** (work_items + state_transitions tables), and **TimescaleDB**
(signal_metrics hypertable). Each write is wrapped in retry-with-backoff;
final failures dead-letter to a Mongo collection. Acceptance: 200 signals
to one component_id within 8 seconds → **1–3** Work Items in Postgres,
**~200** raw signals in Mongo (reduction ratio ≥ 60×).

---

## 2. The four critical rules — which apply this phase

| Rule | This phase? | How we enforce |
|---|---|---|
| 1. Ingestion never blocks on persistence | ✅ existing | Pipeline still uses bounded channel + 503 on full. Phase 2's mechanism unchanged. |
| 2. State transitions are transactional | ⏸ Phase 4 | We only create Work Items in OPEN state this phase; no transitions yet. |
| 3. RCA required for CLOSED | ⏸ Phase 4 | N/A — Closed state isn't reachable until Phase 4. |
| 4. Debounce is atomic via Lua | ✅ NEW | `backend/internal/debounce/script.lua` loaded with SCRIPT LOAD at startup. Single EVALSHA per signal. |

---

## 3. Scope — what we build

### 3.1 New packages

| Package | Responsibility |
|---|---|
| `internal/persist/pg` | pgx v5 connection pool, `WorkItemRepository` (Insert / IncrementSignalCount / Ping). |
| `internal/persist/mongo` | Mongo client, `SignalRepository` (InsertOne / Ping), `DeadLetterRepository` (InsertOne). |
| `internal/persist/redis` | go-redis client, holds the loaded Lua script SHA, `Ping`. |
| `internal/persist/timescale` | Reuses the pgx pool; thin writer for `signal_metrics` hypertable. |
| `internal/debounce` | Thin wrapper around the Redis Lua call. Returns `(workItemID, action, count)`. Has a fallback path for FR-3.6. |
| `internal/processor` | Orchestrates per-signal work: call debouncer, then fan out to all four sinks with retry+backoff; dead-letter on exhaustion. This is what plugs into `pipeline.Pipeline` as the `Processor` func. |

### 3.2 SQL migrations (in `backend/migrations/`)

Forward-only, sequentially numbered, run with `migrate ... up`.

| File | Creates |
|---|---|
| `001_work_items.up.sql` | `work_items` table (id, component_id, component_type, severity, status, signal_count, first_signal_ts, last_signal_ts, mttr_seconds, created_at, updated_at). Indexes on `(status, severity)` and `(component_id, status)`. |
| `001_work_items.down.sql` | DROP TABLE work_items. |
| `002_state_transitions.up.sql` | `state_transitions` audit table (id, work_item_id FK, from_state, to_state, reason, created_at). Index on `(work_item_id, created_at)`. |
| `002_state_transitions.down.sql` | DROP TABLE state_transitions. |
| `003_signal_metrics.up.sql` | `signal_metrics` table + `SELECT create_hypertable('signal_metrics', 'ts')` so it auto-partitions by time. |
| `003_signal_metrics.down.sql` | DROP TABLE signal_metrics (which also drops the hypertable). |

The init script (`docker/postgres/init.sql`) already enables the
timescaledb extension; migrations assume it's present.

### 3.3 Redis Lua script (`backend/internal/debounce/script.lua`)

Exactly the script from `01-architecture.md` §5.3:

```lua
-- KEYS[1] = debounce:{component_id}:work_item
-- KEYS[2] = debounce:{component_id}:count
-- ARGV[1] = candidate_work_item_id (used only if a new window opens)
-- ARGV[2] = window_seconds (10)
-- ARGV[3] = max_signals (100)

local existing = redis.call('GET', KEYS[1])
local count = tonumber(redis.call('GET', KEYS[2]) or '0')

if existing and count < tonumber(ARGV[3]) then
    redis.call('INCR', KEYS[2])
    return {existing, 'JOINED', count + 1}
else
    redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
    redis.call('SET', KEYS[2], '1', 'EX', ARGV[2])
    return {ARGV[1], 'CREATED', 1}
end
```

Loaded with `SCRIPT LOAD` at startup; subsequent calls use `EVALSHA` (one
round-trip, server-cached parse).

### 3.4 Processor flow (the new core)

For every signal off the queue, in order:

1. **Debounce.** Call `debouncer.Process(ctx, signal)` → returns
   `(workItemID, action, count)` where action ∈ {`CREATED`, `JOINED`}.
   On Redis failure: fall through to "always CREATED" with a fresh UUID,
   log the degradation.
2. **Stamp the signal.** Attach `workItemID` to the signal.
3. **Fan out (independently, NOT a distributed transaction):**
   - Mongo: insert raw signal document into `signals`.
   - Postgres: if `CREATED` → `INSERT` new work_items row. If `JOINED`
     → `UPDATE` signal_count + last_signal_ts on the existing row.
   - Timescale: `INSERT` one row into `signal_metrics`.
4. **Each write retries** up to 3 attempts with exponential backoff
   (`cenkalti/backoff/v4`: 100ms, 200ms, 400ms). Final failure pushes a
   dead-letter doc to Mongo `dead_letter` collection.

Errors are *not* fatal for the processor — one sink failing doesn't
block the others. The pipeline counter increments `errors` for any
dead-lettered write.

### 3.5 `/health` upgrade (FR-8.1)

Phase 2's `/health` returned only queue stats. Phase 3 adds a
`dependencies` block that pings each sink with a 500ms timeout. Response
is 503 if **any** critical dep is down (Postgres or Mongo); Redis-down
is `degraded` not critical (we have a fallback path).

```json
{
  "status": "healthy",
  "queue_depth": 47,
  "queue_capacity": 50000,
  "counters": { ... },
  "dependencies": {
    "postgres": {"status": "up", "latency_ms": 2},
    "mongo":    {"status": "up", "latency_ms": 4},
    "redis":    {"status": "up", "latency_ms": 1}
  }
}
```

### 3.6 Config (env, with defaults — added this phase)

| Var | Default | Why |
|---|---|---|
| `DATABASE_URL` | `postgres://ims:ims@localhost:5432/ims?sslmode=disable` | pgx pool DSN |
| `MONGO_URI` | `mongodb://ims:ims@localhost:27017/ims?authSource=admin` | mongo-driver URI |
| `REDIS_ADDR` | `localhost:6379` | go-redis address |
| `IMS_DEBOUNCE_WINDOW_SECONDS` | `10` | FR-3.1 |
| `IMS_DEBOUNCE_MAX_SIGNALS` | `100` | FR-3.1 |
| `IMS_RETRY_MAX_ATTEMPTS` | `3` | FR-8.3 / 01-arch §6.3 |
| `IMS_RETRY_BASE_MS` | `100` | base for exponential backoff |
| `IMS_DEP_PING_TIMEOUT` | `500ms` | per-dep budget on /health |

---

## 4. Implementation order

1. SQL migrations + run `migrate ... up` against the local stack. Confirm
   tables exist via `psql`.
2. `internal/persist/pg` — pool constructor + `WorkItemRepository` with
   Insert/IncrementSignalCount/Ping. Integration test with testcontainers.
3. `internal/persist/mongo` — client constructor + `SignalRepository` +
   `DeadLetterRepository`. Testcontainers.
4. `internal/persist/redis` — client + script loader (`SCRIPT LOAD`).
   Testcontainers.
5. `internal/persist/timescale` — `Insert` into `signal_metrics`. Shares
   pg pool. Test uses the same pg testcontainer.
6. `internal/debounce` — wraps the Redis Lua call; takes the loaded SHA.
   Has the FR-3.6 fallback path. Concurrency test: N goroutines, same
   component_id → assert ≤ ⌈N/100⌉ distinct work_item_ids returned.
7. `internal/processor` — orchestrates everything. Takes interfaces for
   each repo so unit tests can use fakes. Backoff wraps each write.
8. `cmd/ims/main.go` — bring up pools, load the Lua script, build the
   processor, hand it to `pipeline.New(... , processor.Process)`. Upgrade
   `/health` handler with the Pinger list.
9. `scripts/simulate-component-storm.sh` — fire 200 signals at one
   component_id; print row counts from Mongo/Postgres to confirm
   acceptance. (The fuller failure-simulator script is Phase 6.)

---

## 5. Acceptance criteria

- [ ] `migrate -path backend/migrations -database "$DATABASE_URL" up`
      creates 3 tables; `down` then `up` is idempotent.
- [ ] `go test -race ./...` passes, including the testcontainers-backed
      repo tests and the debounce concurrency test.
- [ ] `go vet ./...`, `gofmt -l .` clean.
- [ ] `curl /health` returns 200 with `dependencies` populated; latencies
      under 50ms for each on a healthy stack.
- [ ] **Acceptance demo:** with the stack up and the backend running,
      send 200 signals to `component_id=CACHE_CLUSTER_01` over 8 seconds:
  - Mongo `signals` collection has 200 documents with that component_id.
  - Postgres `work_items` has between 1 and 3 rows for that component_id.
  - Reduction ratio (signals ÷ work_items) ≥ 60×.
- [ ] Killing the Redis container while the backend runs → backend keeps
      accepting signals; `/health` shows `redis: down` and overall status
      `degraded`; work_items still being created (FR-3.6 fallback).
- [ ] Killing the Postgres container → work_item writes go to
      `dead_letter` after 3 retries; backend doesn't crash; Mongo audit
      still receives the raw signals.

---

## 6. Non-goals (do **not** add this phase)

- State machine, transitions, RCA validation, MTTR (Phase 4).
- gRPC ingestion (Phase 5).
- Dashboard / frontend wiring (Phase 5).
- The full failure simulator script (Phase 6 — we only need a tiny
  storm-fire script for acceptance).
- Auto-replay of dead-letter records (NG, see 01-arch §12).

---

## 7. Learning notes (whiteboard check)

After Phase 3, be able to explain unaided:

1. Why is the debounce decision check-then-act, and why does that need
   atomicity? (01-arch §5.2.)
2. Why is the script loaded with `SCRIPT LOAD` once instead of being
   sent as the body of `EVAL` on every call?
3. Why are the four sink writes NOT a distributed transaction? Which is
   the source of truth and why? (01-arch §6.2.)
4. Why does Redis being down keep the system running, while Postgres
   being down dead-letters? What's the asymmetry?
5. Why does `SELECT create_hypertable('signal_metrics', 'ts')` need to
   happen via migration, not at table-create time?
6. Explain the retry sequence: base 100ms, x2, 3 attempts. What's the
   total wall-clock time of a failed write before dead-letter?
   (~700ms: 100 + 200 + 400.)
7. Why is the rate limiter still per-process, not in Redis? (We
   discussed in Phase 2; revisit.)

If any of these is fuzzy, re-read the cited section before writing code.
