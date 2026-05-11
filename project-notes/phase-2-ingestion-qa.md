# Phase 2 — Ingestion & Backpressure: Q&A Study Guide

> **Phase scope:** Build the HTTP ingestion path. `POST /v1/signals`
> accepts JSON, validates, enqueues onto a bounded channel, and a worker
> pool drains it. Add a per-source rate limiter, an upgraded `/health`,
> and a metrics ticker. **Verified:** vegeta sustained 10K req/s for 60s
> with p99 = 1.89 ms.

---

## 1. What we built (one paragraph)

A single HTTP endpoint (`POST /v1/signals`) that takes JSON, validates
it, and pushes it onto a Go channel with a fixed size (50,000 slots).
Twenty worker goroutines pull from that channel and process. If the
channel fills up (i.e., workers are slower than producers), the next
request gets a 503 "queue full" response in microseconds — no waiting.
A per-source rate limiter sits in front to stop one chatty client from
hogging the budget. Every 5 seconds a metrics line prints throughput,
queue depth, and drop count.

The processor is still a no-op (count and discard) — Phase 3 will swap
in real persistence. The point of Phase 2 was to prove the *plumbing*
can handle 10K/sec without crashing.

---

## 2. The fundamentals

### Q: What's a goroutine? How is it different from a thread?
**A:** A goroutine is a function running concurrently, managed by the
Go runtime (not the OS). They're *very* cheap: a goroutine starts at
~2 KB of stack and the runtime grows/shrinks it as needed. Threads are
~1–2 MB each. So you can have **hundreds of thousands of goroutines**
in one process, whereas threads max out in the low thousands.

Created by prefixing a call with `go`:
```go
go worker(ctx)  // starts worker(ctx) in a new goroutine, returns instantly
```

### Q: What's a Go channel?
**A:** A typed, thread-safe queue. `chan int` is a channel of integers.
You write with `ch <- 42` and read with `x := <-ch`. Channels coordinate
goroutines: one goroutine produces values, another consumes them.
Channels can be **bounded** (`make(chan int, 100)`) or **unbounded**
(`make(chan int)`).

### Q: What does "bounded channel" mean and why does the size matter?
**A:** Bounded = the channel has a max number of unconsumed items. Once
that many are queued, the next send either *blocks* (default) or *fails*
(if we use a non-blocking `select`). Size matters because:
- Too small → 503s under normal bursts.
- Too big → memory bloats; the queue masks slow downstream sinks until
  it's "too late" (workers can't catch up).

Ours is 50,000 = 5 seconds of nominal load at 10K/sec. Enough to absorb a
brief stall, small enough that 5s after a real outage we start back-pressuring
upstream.

### Q: What's the `select` statement in Go?
**A:** A multi-way switch over channel operations. The runtime picks the
first case whose channel is ready; if multiple are ready, it picks
randomly. The pattern we care about is:

```go
select {
case ch <- value:   // try to send
    // succeeded
default:            // ← this fires if the send would block
    // queue was full
}
```

Adding `default` makes the whole `select` non-blocking. **This is the
entire backpressure mechanism.**

### Q: What does "backpressure" mean?
**A:** Backpressure is what a system does when downstream can't keep up
with upstream. Three policy choices:
1. **Block** — make the producer wait. Bad for HTTP because the caller
   holds a connection.
2. **Buffer forever** — unbounded queue. Crashes under sustained
   overload (OOM).
3. **Reject and tell them to retry** — what we do. HTTP 503 with
   `Retry-After`. The producer (an observability agent) is expected to
   back off and try again.

### Q: What's a "worker pool"?
**A:** A fixed number of goroutines that loop forever consuming from a
shared channel. Each worker is interchangeable — no IDs, no per-worker
state. We start `NumCPU() * 2` of them. They block on `<-channel`
until something arrives; the Go runtime parks them off-CPU during the
wait, so idle workers cost ~nothing.

### Q: Why `NumCPU() * 2` workers, not 100 or 10,000?
**A:** Workers are **I/O-bound** (waiting on DBs and the network), not
CPU-bound. Starting 2× the CPU count is the standard heuristic for
I/O-bound work: while one worker waits, another runs. More than 2×
gives diminishing returns and adds context-switching cost. (For pure
CPU work the heuristic flips: 1× NumCPU.)

### Q: What's a `context.Context` and why does every function take one?
**A:** A `Context` carries (a) a cancellation signal, (b) a deadline,
(c) a values map. It's how Go propagates "stop what you're doing" through
a call stack. When the user hits Ctrl-C, our `signal.NotifyContext`
cancels the root context; every goroutine that's watching `<-ctx.Done()`
sees it and exits. Without context propagation, graceful shutdown
becomes a forest of bespoke `quit` channels.

### Q: What's `sync/atomic` and why use it for counters?
**A:** Stdlib operations on integers that are guaranteed to be safe
across goroutines without a mutex. `atomic.Uint64` with `.Add(1)` and
`.Load()` is what we use for the accepted/processed/dropped counters.
Why atomic and not mutex: mutex would serialize all counter updates,
killing throughput at 10K/sec. Atomics use CPU-level compare-and-swap
instructions and don't block.

### Q: What's a "data race"?
**A:** When two goroutines access the same variable, at least one is
writing, and there's no synchronization between them. The Go race
detector (`go test -race`) instruments your code to find these. **Any**
data race in Go is undefined behavior, *not* "slightly stale value". We
hit one in Phase 2 (rate limiter's `lastSeen` field) and fixed it with
`atomic.Int64`.

### Q: What's a token-bucket rate limiter, in plain language?
**A:** Imagine a bucket that fills with tokens at a constant rate (say,
1000 tokens per second), and has a maximum capacity (the "burst"). Every
request takes one token. If there are tokens, request goes through. If
the bucket is empty, request is rejected (429). The rate is the steady
state; the burst is how much instantaneous spike we allow.

`golang.org/x/time/rate` implements this. We create one bucket per
source IP.

### Q: What's an "atomic" operation in CPU/memory terms?
**A:** An operation the CPU guarantees happens "all at once" from the
perspective of other cores — no other core can see a half-completed
state. On modern x86/ARM, instructions like `LOCK ADD` or `LDREX/STREX`
provide this. `atomic.Int64.Add(1)` compiles down to one such
instruction. By contrast, `i++` is actually three steps (load, add,
store) and not safe across goroutines.

---

## 3. The tech we used

### Q: What is `gin.Context`?
**A:** Gin's per-request object. Bundles the http.Request, http.ResponseWriter,
parsed parameters, query strings, and helpers like `c.JSON(200, body)`.
Every handler signature is `func(c *gin.Context)`.

### Q: What is `c.ShouldBindJSON(&sig)`?
**A:** Reads the request body, parses it as JSON, populates `sig`'s fields
based on the `json:"..."` struct tags. Returns an error if the JSON is
malformed (we return 400) but does NOT enforce business validation —
that's why we call `sig.Validate()` after.

### Q: What's `httptest.NewRecorder()` in the tests?
**A:** A fake `http.ResponseWriter` from the stdlib. Lets you drive a
handler without spinning up a real server. After `r.ServeHTTP(w, req)`,
`w.Code` is the response status, `w.Body` is the bytes written. Standard
Go HTTP testing pattern.

### Q: What is `golang.org/x/time/rate`?
**A:** An "extension" stdlib package (in the `golang.org/x/` namespace).
Implements a token-bucket limiter. `rate.NewLimiter(rate.Limit(1000), 2000)`
makes one that refills at 1000/sec, capped at 2000 tokens. `.Allow()`
returns `true` if a token was available and consumed it.

### Q: What's `google/uuid`?
**A:** A library that generates and parses UUIDs (universally unique
identifiers). We use `uuid.New()` to mint signal IDs when the client
omits one. ~50 nanoseconds per call.

### Q: What is vegeta?
**A:** An HTTP load-testing tool written in Go. You give it a target
list and a rate (`-rate=10000/1s`) and a duration (`-duration=60s`); it
fires requests at that exact rate and reports latency percentiles, error
rates, and a histogram. Much more accurate than `ab` or naive curl loops
because it pre-creates connections and paces sends precisely.

### Q: What's `gin.Recovery()` middleware?
**A:** Wraps every handler in a panic-recovery `defer`. If a handler
panics (nil pointer, unexpected nil map, etc.), Gin catches it, logs
the stack trace, and returns 500 instead of crashing the whole server.
You always want this in production.

---

## 4. The design decisions

### Q: Why is `POST /v1/signals`, not `POST /signals`?
**A:** URL versioning. Future-you wants to be able to ship `/v2/signals`
with a breaking schema change while old clients keep using `/v1`. Costs
nothing to start with `/v1`.

### Q: Why does the handler return 202, not 200?
**A:** 200 means "done, here's the result." 202 means "**accepted** for
processing, but I haven't actually done anything yet." That's literally
true here — we've enqueued, not processed. The HTTP spec exists for a
reason; use it correctly.

### Q: Why 503 when the queue is full, not 429?
**A:** Different semantics:
- **429 Too Many Requests** = "you, specifically, are sending too fast."
  That's our rate-limiter response.
- **503 Service Unavailable** = "I, the server, can't handle it right
  now." That's our queue-full response — every caller, not just one.

### Q: Why include a `Retry-After` header on the 503?
**A:** Standard HTTP convention. A well-behaved client (and our future
observability agents) reads `Retry-After` and waits that long before
retrying. Without it, naive clients hammer in tight loops.

### Q: Why does the handler set `signal_id` server-side instead of
requiring the client?
**A:** Convenience — clients (a `curl`, the failure simulator) shouldn't
need to generate UUIDs to test. If the client *does* provide one (e.g., a
sophisticated agent doing idempotent retries), we honor it. `ApplyDefaults`
fills in the gap.

### Q: Why store the rate-limiter buckets in an in-process map?
**A:** Simplicity and speed. Lock-free atomic on the hot path. Caveat:
this state lives in *one* process — if we run two ingestion replicas,
they'd each have their own limiter and a chatty source could double-dip.
Architecture §3.1 says we run one replica for v1. Phase 3+ might move
this to Redis if we scale out.

### Q: Why a sweeper for the rate-limiter map?
**A:** Without one, every unique IP we ever see leaves a `bucket` in the
map forever. Memory leak. The sweeper evicts buckets that haven't been
touched for `idle` (5 min default).

### Q: Why `atomic.Int64` for `lastSeen` instead of taking the mutex?
**A:** The map's mutex is taken on every bucket lookup. Bumping
`lastSeen` *also* under the mutex would force a write lock per request,
turning the limiter into a global serialization point. Storing
`lastSeen` as `atomic.Int64` (UnixNano) lets the hot path update it
lock-free. The mutex only protects the map's structure.

### Q: Why do we drain HTTP first, then the pipeline, on shutdown?
**A:** Ordering matters:
1. If we closed the pipeline first, in-flight HTTP handlers would try to
   send on a closed channel and panic.
2. `srv.Shutdown(ctx)` blocks until all in-flight handlers return. Once
   it returns, no goroutine can be inside `Submit`.
3. THEN we close the channel and let workers drain.

This is in `cmd/ims/main.go` and is one of the more easily-screwed-up
parts of a Go service.

### Q: Why does the metrics ticker compute *rates* (per second) and not
just totals?
**A:** Totals always grow; you can't eyeball "are we keeping up?" from
a number that increases. Rate (accepted/sec last interval) is what
operators look for. The ticker computes it by diffing the cumulative
counter against the previous tick.

### Q: Why does `/health` flip to 503 at >95% queue full, not 100%?
**A:** Early warning. By 95% we're 50ms from saturation at 10K/sec. If
the operator (or load balancer) only sees 503 at 100%, they react too
late — the next request is already failing. 95% gives them a heartbeat.

---

## 5. Tradeoffs

### Q: An unbounded channel would never 503. Why didn't we use one?
**A:** It would 503 eventually — by OOM-killing the process. Bounded +
explicit rejection is *honest*: we tell the caller "we can't handle you
right now" instead of pretending and dying. Memory matters; processes
running out of memory take down everything else with them.

### Q: We're rejecting requests when the queue is full. What if those
requests were P0 incidents?
**A:** Fair concern. v1 treats all signals equally. Two future
improvements:
- **Priority queue**: P0 signals get a separate, smaller, dedicated
  channel that workers prefer.
- **Shed lower priorities first**: instead of a flat 503, drop P3s
  pre-emptively to free room for P0s.

Neither is in v1 — explicit non-goal to keep the design defensible
("one mechanism, easy to explain").

### Q: A Kafka topic between the handler and workers would survive
process restarts. Why not use that?
**A:** Kafka adds a container, a driver, a deployment dependency, plus
~5–10ms broker round-trip per signal. Architecture §4.4 defends the
in-process channel: at 10K/sec on one box, channel wins on every axis
*except* durability. Signals are inherently lossy (the producer
fire-and-forgets), so durability is the requirement we explicitly don't
have. **Swap is local** — Phase 5's interface is `chan Signal`, replace
the channel with a Kafka consumer and the rest of the code doesn't
change.

### Q: Per-source rate limiting protects against one chatty IP, but a
botnet from 1000 IPs each hitting 1000/s = 1M/s. What stops that?
**A:** Nothing in our limiter. The system-wide cap is the channel
capacity + worker throughput. At sustained 1M/s with 50K queue and 10K
processing rate, we'd 503 ~99% of requests — exactly the design.
"Distributed DoS protection" is upstream's job (CDN, WAF). v1
explicitly out of scope.

### Q: We deleted in-flight signals on a slow shutdown. Should we
persist the queue to disk?
**A:** Could. Cost: every signal write goes to disk (latency) or a
periodic snapshot (still loses some). Architecture §9 marks this as
"in-flight signals: best-effort" — the producer should retry. For a
v1 demo, the tradeoff is fine. Production might want a WAL-style
buffer.

---

## 6. Interview gotchas

> *Tight answers; expand from the sections above if pressed.*

**1. "Walk me through what happens when I POST a signal."**
1. Gin router matches `POST /v1/signals` → rate-limit middleware.
2. Limiter checks the bucket for this IP. Token available → continue.
3. Handler calls `ShouldBindJSON` to parse, then `Validate()` for
   business rules.
4. Handler runs `ApplyDefaults(time.Now())` to mint a `signal_id` if
   missing.
5. Non-blocking `pipe.Submit(sig)` → returns true (queue had room).
6. Handler returns 202 with the signal_id.
7. A worker picks it up from the channel, runs the (Phase 2 no-op)
   processor, increments `processed`.

End-to-end p99 measured at **1.89 ms** under 10K/sec sustained load.

**2. "Why is the channel the entire backpressure story?"**
Bounded channel + non-blocking send (`select` with `default`) means
producers either fit or get rejected in nanoseconds. No buffering games,
no separate "are we busy?" flag, no probability — pure mechanical
"fits or doesn't." Easy to reason about, easy to defend.

**3. "How do you prevent silent signal loss?"**
The invariant: `accepted + dropped == total submissions`, and every
accepted signal is processed exactly once. We **test** this invariant
in `pipeline_test.go` under `-race`. Dropped is not silent — the caller
got a 503 with `Retry-After`.

**4. "What if a worker panics mid-signal?"**
`workerLoop` has a top-level `recover()`. The panicking signal is lost
(logged with `signal_id`), the worker exits, the other 19 keep
draining. Phase 6 might respawn the worker; v1 accepts losing one of
20 worker slots.

**5. "Why is the rate limiter not in Redis?"**
For v1 we deploy one ingestion replica, so per-process state is
sufficient. Per-replica state in Redis would survive replica restarts
and span replicas, but adds a Redis round-trip per request (~1ms).
We pay that for debounce (Phase 3) but not for rate-limiting.

**6. "How did you avoid a data race in the rate limiter?"**
Map structure is protected by `RWMutex` (writers create new buckets,
readers do lookups). Within a bucket, `lim.Allow()` is internally
lock-free (that's `x/time/rate`'s contract), and `lastSeenNano` is
`atomic.Int64` so the hot path can update it without taking the map's
write lock. Verified with `go test -race`.

**7. "Why did you separate the `Submitter` interface from the
`Pipeline` struct?"**
"Define small interfaces where you consume them" (Go idiom). The
handler only needs `Submit(model.Signal) bool`. Defining `Submitter` in
the `ingest` package lets us inject a fake in tests without depending
on the real `pipeline.Pipeline` (decouples).

**8. "How did you size the queue to 50,000?"**
~5 seconds of nominal load at 10K/sec. Big enough to absorb a multi-second
DB stall (Phase 3); small enough that we 503 before memory is a problem
(50K × ~400 bytes/signal = ~20 MB). The number is a tunable; default
is documented but env-overridable.

**9. "What's the difference between `Allow()` and `Wait()` on
`rate.Limiter`?"**
- `Allow()` is non-blocking — returns true if a token was available,
  false otherwise.
- `Wait(ctx)` blocks until a token is available (or ctx is cancelled).

We use `Allow()` because **the handler must never block** (FR-1.4).
Blocking the handler ties up the HTTP server's goroutine pool.

**10. "If a single source IP is being rate-limited but legitimate
high-volume traffic shares that NAT IP, you have a problem. What do
you do?"**
Legit issue. Options:
- Trust an `X-Forwarded-For` header from a known proxy (Gin can do
  this) and rate-limit on the *real* origin IP.
- Higher rate limit if the request carries a known API key.
- Bypass the limiter entirely for trusted networks.

v1 keys on `c.ClientIP()`, which respects `X-Forwarded-For` if Gin is
configured to trust the proxy. Not done in this phase but documented as
a known limitation.

**11. "Walk me through what `select { case ch <- v: ...; default: ... }`
compiles to."**
The Go runtime evaluates each case's channel state. If the send would
not block (i.e., room in the buffer or a receiver waiting), it picks
the `ch <- v` arm. Otherwise, it picks `default`. The non-blocking
property comes entirely from the presence of `default` — without it,
the `select` blocks until one case is ready. This is the cleanest
non-blocking-send idiom in Go.

**12. "How would you scale this beyond one replica?"**
Move the rate-limiter buckets to Redis (so all replicas share state).
The channel becomes Kafka or NATS. Workers consume from the broker
across replicas, debounce + persistence (Phase 3) is already designed
for multi-replica via Redis Lua atomic ops. The HTTP layer is stateless,
so you put it behind any load balancer.

---

## 7. Things you should be able to do after Phase 2

- [ ] Explain (without notes) why 50K is the queue size and `NumCPU()*2`
      is the worker count.
- [ ] Run the load test yourself: `IMS_RATE_LIMIT_RPS=20000 go run ./cmd/ims`
      in one terminal, `./scripts/load-test.sh` in another.
- [ ] Read the vegeta report and tell me what p50, p90, p99 mean and
      why p99 is the one that matters for SLOs.
- [ ] Lower `IMS_QUEUE_CAPACITY=100` and re-run the load test. Watch the
      drop count climb. Explain why.
- [ ] Lower `IMS_WORKER_COUNT=1` and re-run. Now drops climb even with
      a big queue. Explain the bottleneck.
- [ ] Crash a worker by adding `panic("test")` in `NoopProcessor`.
      Confirm the server keeps running, other workers keep draining.
- [ ] Open `internal/pipeline/pipeline.go` and trace one signal from
      `Submit` to processor invocation.

If all of those feel comfortable, Phase 2 is solid in your head.
