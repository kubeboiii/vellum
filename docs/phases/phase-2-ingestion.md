# Phase 2 — Ingestion & Backpressure

> **Day:** 2 of 7
> **Depends on:** Phase 1 (Foundation) complete — Gin server on `:8080`, docker stack reachable.
> **References:** `00-master-prd.md` §4.1, §4.2, §4.8, §5.1, §5.2; `01-architecture.md` §4, §11.
> **Out of scope:** Debounce (Phase 3), persistence (Phase 3), workflow (Phase 4), gRPC (Phase 5).

---

## 1. Goal in one sentence

Stand up the HTTP ingestion path end-to-end so the backend can sustain
**10,000 signals/sec for 60 seconds with p99 latency under 50ms** (NFR-1.1),
returning `503` with `Retry-After` the instant the in-memory queue saturates
(FR-1.5 / NFR-2.1).

There is no persistence yet. Workers consume from the channel and *discard*
the signal after counting it. Phase 3 swaps the discard for the
debounce-and-fan-out logic.

---

## 2. Scope — what we build

### 2.1 New packages

| Package | Responsibility |
|---|---|
| `internal/model` | `Signal` struct, JSON tags, enum types (`ComponentType`, `Severity`), `Validate()` method. |
| `internal/pipeline` | Bounded `chan model.Signal`, worker pool, graceful drain on ctx cancel, accepted/processed/dropped counters. |
| `internal/ingest` | Gin handler for `POST /v1/signals` (non-blocking enqueue) and a per-source token-bucket rate-limit middleware. |
| `internal/obs` | Real `/health` (queue depth + dep status placeholder — DBs come Phase 3) and the 5-second metrics ticker. |

`internal/persist/*`, `internal/debounce`, `internal/workflow`, `internal/alert`
stay empty until their phase fires. Don't import them.

### 2.2 Endpoints

| Method | Path | Body | Codes |
|---|---|---|---|
| `POST` | `/v1/signals` | JSON signal (§4.2 schema) | `202` accepted, `400` validation, `429` rate-limited, `503` queue full |
| `GET` | `/health` | — | `200` healthy, `503` if queue saturated (>95% full) |

Wire format for `/v1/signals` mirrors `00-master-prd.md` §4.2:

```json
{
  "signal_id": "uuid (optional; server generates)",
  "component_id": "RDBMS_PRIMARY_01",
  "component_type": "RDBMS",
  "severity": "P0",
  "timestamp": "2026-05-12T03:14:15Z",
  "source": "datadog",
  "payload": { "...free-form..." }
}
```

Response on success:
```json
{ "status": "accepted", "signal_id": "..." }
```

Response on queue full:
```json
{ "error": "ingestion queue full", "retry_after_ms": 100 }
```
with `Retry-After: 1` header (seconds, rounded up).

### 2.3 Config (env, with defaults)

| Var | Default | Why |
|---|---|---|
| `VELLUM_HTTP_ADDR` | `:8080` | bind address |
| `VELLUM_QUEUE_CAPACITY` | `50000` | 5s of nominal at 10K/sec (01-arch §4.2) |
| `VELLUM_WORKER_COUNT` | `NumCPU()*2` | I/O-bound oversub (01-arch §4.3) |
| `VELLUM_RATE_LIMIT_RPS` | `1000` | per-source default (FR-1.6) |
| `VELLUM_RATE_LIMIT_BURST` | `2000` | burst tolerance |
| `VELLUM_METRICS_INTERVAL` | `5s` | stdout metrics line cadence (FR-8.2) |
| `VELLUM_SHUTDOWN_TIMEOUT` | `30s` | drain deadline (NFR-2.4) |

---

## 3. The four design rules — how this phase respects them

Only rule #1 matters this phase:

> **Rule 1 — Ingestion never blocks on persistence.**

Concrete enforcement:

- The handler does **one** thing after validation: a non-blocking `select`
  send onto the bounded channel. If the `default` arm fires, return 503
  immediately. No `time.Sleep`, no retry, no logging on the hot path beyond
  counter increments.
- Workers run in their own goroutines. The HTTP handler never waits for
  one of them to free up.
- Rule 4 (debounce atomic) is N/A this phase (Phase 3).
- Rules 2 & 3 (transactional transitions, RCA enforcement) are N/A this
  phase (Phase 4).

---

## 4. Implementation order

1. `internal/model/signal.go` — types + `Validate()`. Tests for the validator.
2. `internal/pipeline/pipeline.go` — `New(cap, workers, processor)`, `Start(ctx)`,
   `Submit(sig) bool`, `Stop(ctx)`. Counters via `sync/atomic`. Tests for
   accept/drop semantics and graceful drain.
3. `internal/ingest/handler.go` — Gin handler taking a `*pipeline.Pipeline`.
   Tests using `httptest`.
4. `internal/ingest/ratelimit.go` — `golang.org/x/time/rate` per-source bucket
   keyed by IP, sweeper to evict idle buckets. Tests with synthetic IPs.
5. `internal/obs/health.go` — handler reading queue depth from the pipeline.
6. `internal/obs/metrics.go` — `Ticker.Run(ctx)` printing the structured
   line from FR-8.2.
7. `cmd/vellum/main.go` — wire pipeline + handler + middleware + ticker;
   replace the placeholder `/health`.
8. `scripts/load-test.sh` — vegeta script. Target: 10K rps, 60s, p99 < 50ms,
   error rate < 1% (allowing some 429/503 noise from the rate limiter).

---

## 5. Acceptance criteria (the build is "done" when…)

- [ ] `go test -race ./...` passes, including a concurrency test that fires
      N goroutines submitting signals and asserts `accepted + dropped == N`
      with no double-counting.
- [ ] `go vet ./...`, `gofmt -l .` clean.
- [ ] `curl -X POST http://localhost:8080/v1/signals -d '{...}'` returns
      202 with a `signal_id`.
- [ ] `curl http://localhost:8080/health` returns 200 with `queue_depth`,
      `queue_capacity`, and `dependencies` (placeholder structure ready for
      Phase 3 to fill).
- [ ] The metrics ticker prints a single line every 5s in the documented
      format (`[metrics] accepted=… processed=… queue=… ...`).
- [ ] `./scripts/load-test.sh` reports:
  - sustained ≥ 10,000 req/s for 60 seconds
  - p99 latency ≤ 50 ms
  - success rate ≥ 99% (allowing minor 429/503)
- [ ] Sending SIGINT during load drains the queue cleanly within
      `VELLUM_SHUTDOWN_TIMEOUT` and prints a final metrics line.

---

## 6. Non-goals (do **not** add this phase)

- Anything that reads or writes Postgres, Mongo, Redis, or TimescaleDB.
  Workers discard signals after counting. The 4 stores are touched in Phase 3.
- gRPC ingestion. Same pipeline contract, but Phase 5.
- Debounce logic. Phase 3.
- Auth or per-tenant rate limits. Out of scope (NG1, NG2).

If you find yourself reaching for any of these, stop.

---

## 7. Learning notes (you must be able to explain on a whiteboard)

After Phase 2, you should be able to answer, unaided:

1. Why is a bounded Go channel the entire backpressure mechanism?
   (Reference: 01-arch §4.2.)
2. Why `NumCPU()*2` workers and not `NumCPU()` or `1000`?
   (Reference: 01-arch §4.3.)
3. Why 50K channel capacity, not 5K or 500K?
   (5s of nominal load at 10K/sec; trade-off between burst absorption and
   memory.)
4. Why is the rate limiter per-source, not global?
   (FR-1.6: one chatty source mustn't starve others; global limit would
   conflict with the 10K/sec system target.)
5. Why 503 with `Retry-After` rather than blocking the client?
   (FR-1.5 + NFR-2.1: ingestion handlers must not hold connections; client
   retries with backoff.)

If any of these is fuzzy, re-read the cited section before writing code.
