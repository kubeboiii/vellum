# Phase 5 — gRPC + Frontend: Q&A Study Guide

> **Phase scope:** Second ingestion protocol (gRPC server-streaming
> on `:9090`, sharing the in-process pipeline with HTTP), Next.js 14
> dashboard (live feed / detail / RCA form), CORS middleware, paginated
> signals listing, and a 40001-Postgres-error → 409-Conflict mapping
> for serialization races.
>
> **Acceptance result:** 10 signals streamed via gRPC → workflow walk-through
> via REST → RCA submission via REST → status=CLOSED with MTTR populated.

---

## 1. What we built (one paragraph)

A second way to put signals into the system (gRPC streaming) and a
human-facing way to *manage* them (Next.js dashboard). Both rest on
what Phases 2-4 already built: gRPC handlers push to the same bounded
channel; the dashboard reads from the same Postgres + Mongo. The trick
is that we added a second ingestion path *without changing the
downstream code at all*. The dashboard is three pages — live feed,
incident detail, RCA form — that hit the existing REST API. We also
added a small CORS middleware (so the browser can talk to the
backend) and a paginated `GET /v1/incidents/:id/signals` endpoint
(so the detail page can show the raw audit log).

---

## 2. The fundamentals

### Q: What is gRPC?
**A:** A remote-procedure-call framework that uses HTTP/2 as transport
and Protocol Buffers (protobuf) as the wire format. You define a
service in a `.proto` file; codegen produces server stubs (in our
case, Go) and client stubs (Go, TypeScript, Java, etc.). RPC calls
become method invocations on the client object.

### Q: What's "Protocol Buffers" (protobuf)?
**A:** A binary serialisation format with a schema. You write messages
in `.proto` files (typed fields with numeric IDs); the protoc compiler
generates language-specific code. Smaller and faster than JSON
because it's binary, schema-aware, and doesn't include field names on
the wire — just the numeric IDs.

### Q: What does "server-streaming" / "bidi" mean in gRPC?
**A:** Four RPC modes:
- **Unary**: one request → one response (like HTTP).
- **Server-streaming**: one request → stream of responses.
- **Client-streaming**: stream of requests → one response.
- **Bidi-streaming**: stream of requests → stream of responses.

Our `IngestSignals` is **bidi**: the client streams `Signal` messages,
the server streams `Ack` messages back. One TCP/HTTP2 connection,
many signals.

### Q: Why bidi-stream and not just many unary calls?
**A:**
- **Connection amortisation** — one TLS/TCP handshake instead of one
  per signal. At 10K/sec that handshake overhead would dwarf the
  actual work.
- **Backpressure** — the server controls when it reads from the
  stream. If the pipeline is full, the server can slow its reads
  (HTTP/2 flow control kicks in) before even returning a REJECTED ack.
- **Ordered acks** — the client sees Ack[i] before Signal[i+1] is
  processed, so partial-stream failures are debuggable.

### Q: What's the difference between gRPC and REST?
**A:**
- **REST**: HTTP/1.1 (usually), JSON, URL paths + verbs (GET/POST/...),
  open in any browser, every other developer knows it.
- **gRPC**: HTTP/2, binary protobuf, RPC method calls, needs codegen,
  browser support is poor (needs grpc-web proxy).

Use gRPC for service-to-service when throughput matters. Use REST for
public APIs, browser clients, and ad-hoc curl debugging.

### Q: What's `buf`?
**A:** A modern frontend for protoc. Two things it gives you:
- **Linting** — enforce naming conventions (`buf lint`).
- **Code-gen workflow** — `buf generate` reads `buf.gen.yaml` and runs
  the right plugins. No remembering all the `--go_out`, `--go-grpc_out`
  flags.

It can also pull plugins from a remote registry (buf.build), but we
use local plugins for reproducibility.

### Q: What's a "Server Component" vs "Client Component" in Next.js
App Router?
**A:**
- **Server Component** (the default): runs on the server, output is
  sent as HTML. Cannot use `useState`, `useEffect`, `onClick`.
  Can `await` database calls directly.
- **Client Component**: prefix the file with `'use client';`. Runs in
  the browser. Has all the React hooks. Cannot directly use server
  resources.

Use Server Components by default; reach for `'use client'` only when
you need interactivity (forms, polling, state).

### Q: Why does our live feed use `'use client'`?
**A:** Because it polls with `setInterval`. `setInterval` is a
browser API; Server Components run once, server-side, and return
static HTML. To re-render every 2 seconds we need client-side state +
effect.

### Q: What's `useEffect(() => { ... }, [])` actually do?
**A:** Run the function after the component first renders. The empty
dependency array `[]` means "only run once" — without it, the effect
re-runs on every render. The returned cleanup function fires when the
component unmounts (or when deps change). We use it to start the poll
interval on mount and clear it on unmount.

### Q: What's CORS?
**A:** Cross-Origin Resource Sharing. By default a browser refuses to
let JavaScript on `localhost:3000` call an API on `localhost:8080`
because they're "different origins." CORS is the server-side opt-in:
the API responds with `Access-Control-Allow-Origin: <origin>` headers
saying "yes, this origin is allowed." The browser checks; if missing,
it blocks the response.

There's also a **preflight** for non-simple requests: the browser
first sends an `OPTIONS` request asking "is this method+headers
allowed?"; only on a 2xx with the right `Access-Control-*` headers
does the actual request proceed.

### Q: What's a "preflight" request?
**A:** An automatic `OPTIONS` request the browser makes before a
non-simple cross-origin request (anything with a custom header, a
JSON body, or a non-GET/POST method). Our backend returns 204 + the
CORS headers; the browser then issues the real request.

### Q: What's `NEXT_PUBLIC_*` env vars?
**A:** Next.js injects env vars prefixed with `NEXT_PUBLIC_` into the
browser bundle at build time. `process.env.NEXT_PUBLIC_API_BASE` in
our client code becomes the literal string in the compiled JS. Vars
without that prefix are server-only — accessing them in Client
Components evaluates to `undefined`.

### Q: What's `bufconn`?
**A:** An in-memory `net.Listener` from `google.golang.org/grpc/test/bufconn`.
Lets you spin up a gRPC server in tests without binding a real TCP
port. Each test gets its own in-process channel between client and
server — no port collisions, no network stack flakiness.

---

## 3. The tech we used

### Q: What is `protoc-gen-go`?
**A:** The protoc plugin that generates Go message types from `.proto`
files. Produces a `*.pb.go` with one struct per `message`, getters
for every field, and `Marshal`/`Unmarshal` methods. Installed via
`go install google.golang.org/protobuf/cmd/protoc-gen-go@latest`.

### Q: What is `protoc-gen-go-grpc`?
**A:** Companion plugin that generates the gRPC service stubs (the
`SignalServiceServer` interface, `RegisterSignalServiceServer`, the
client). Lives in a separate binary because gRPC is one of *many*
possible RPC frameworks; the protobuf compiler doesn't bake it in.

### Q: What's `grpc.NewServer()` + `RegisterSignalServiceServer()`?
**A:** Standard gRPC server lifecycle in Go. `grpc.NewServer()`
returns an empty server; `RegisterSignalServiceServer(srv, impl)`
attaches our service implementation; `srv.Serve(listener)` starts
accepting. `srv.GracefulStop()` drains in-flight RPCs.

### Q: What's `*pgconn.PgError`?
**A:** The error type pgx returns when Postgres rejects a query.
`.Code` holds the SQLSTATE (e.g., "40001" for serialization failure,
"23505" for unique constraint violation). We use `errors.As` to pull
it out and elevate specific codes to our domain sentinels.

### Q: What's `useRouter` from `next/navigation`?
**A:** Next.js's hook for programmatic navigation in App Router
Client Components. `router.push("/incidents/123")` navigates there
without a full page reload. We use it on the RCA form to jump back
to the detail page after successful submission.

### Q: What's `tabular-nums` Tailwind class?
**A:** Forces digits to be the same width — important for tables of
numbers (signal counts, MTTRs) so they line up vertically. Without
it, "11" is narrower than "00" in most fonts and the column wobbles.

---

## 4. The design decisions

### Q: Why does the gRPC server share the same pipeline as HTTP?
**A:** FR-1.3 in the PRD: "Both endpoints share the same in-process
ingestion pipeline. There is exactly one downstream path regardless
of protocol." Concretely: both `internal/ingest/handler.go` (HTTP)
and `internal/ingest/grpc.go` (gRPC) implement code paths that call
`pipeline.Submitter.Submit(sig)`. That's the *one* path. Debounce,
fan-out, alerts, dead-letter — none of it knows whether the signal
came in over HTTP or gRPC.

This is a load-bearing design choice. It means:
- A bug fix in the processor benefits both protocols immediately.
- Phase 6's chaos tests don't have to be doubled up per protocol.
- Adding a third protocol (Kafka consumer? webhook?) is the same
  pattern: write a thin adapter that calls Submit.

### Q: Why is `Ack_ACK_STATUS_QUEUE_FULL` an ack, not an error?
**A:** gRPC has two failure shapes: in-band (ack with a status field)
and out-of-band (RPC error code like `RESOURCE_EXHAUSTED`). We use
in-band because:
- The client gets one ack per signal in stream order. They can match
  which signal was rejected.
- An out-of-band RESOURCE_EXHAUSTED would kill the whole stream,
  forcing the client to reconnect and replay every signal. The ack
  lets them continue.

### Q: Why is the RCA form's validation duplicated client-side?
**A:** Server-side validation (`model.RCA.Validate`) is the source of
truth (CLAUDE.md rule 3: "this rule lives in exactly one place").
Client-side validation is a *user-experience mirror* — it gives the
user immediate feedback ("min 20 chars") without a round trip. If the
mirror drifts, the server still rejects bad input. The mirror is a
nice-to-have; the gate is the server.

### Q: Why hand-roll CORS instead of using `gin-contrib/cors`?
**A:** ~30 lines of straightforward middleware vs adding another
dependency. Our needs are tiny: one origin (the dashboard), the
standard methods, Content-Type header. The dep would do exactly the
same thing with more config knobs. For a 7-day demo with one
deployment shape, inline beats lib.

### Q: Why is the signals endpoint a separate route from the detail
endpoint?
**A:** Pagination + cardinality. The detail page might have one
work_item with 5,000 raw signals. Returning all of them inline in
`GET /v1/incidents/:id` would balloon the response (FR-7.2: "complete
list of linked raw signals, paginated, default 50/page"). Separate
endpoint = separate cache headers (if we ever add caching),
separate paging cursor, separate rate limit if needed.

### Q: Why are UUIDs in Mongo responses returned as strings, not BSON
Binary?
**A:** The Mongo driver decodes UUIDs into `bson.Binary{Subtype:4,Data:[]byte}`
by default. When the API JSON-marshals that, the frontend gets a
`{Subtype, Data}` object instead of a `"uuid-string"`. We do a
two-pass decode: first into a typed struct (UUIDs come out as
`uuid.UUID`), then merge the typed values back into the map before
returning. ~µs per row, totally invisible to the frontend.

### Q: Why does the API return 409 (not 500) for SQLSTATE 40001?
**A:** 40001 is "serialization failure" — Postgres detected that
under SERIALIZABLE isolation, the current transaction would produce
a result inconsistent with some commit order of all concurrent
transactions, so it aborts. **The abort is correct behaviour.** A 500
implies the server is broken; 409 Conflict tells the client "retry
the same request" and that's exactly what they should do. The
processor's bursty `IncrementSignalCount` UPDATEs racing with a
workflow tx are a normal contention pattern, not a bug.

### Q: Why is the workflow engine's tx SERIALIZABLE but the
processor's UPDATE is NOT?
**A:** Different contention patterns:
- Workflow: low frequency (human-driven), multi-statement, needs
  phantom-read protection on the audit table → SERIALIZABLE makes
  sense.
- Processor: high frequency (10K/sec), single-statement UPDATE on one
  row → Postgres's per-row locks at READ COMMITTED already serialize
  the writes; SERIALIZABLE would add 40001 churn for zero correctness
  win.

The "cost" is that the two paths can sometimes 40001 each other —
which is why the API maps it to 409 (retry).

---

## 5. Tradeoffs

### Q: We picked client-side polling over WebSocket. What did we give up?
**A:**
- **Latency** — WebSocket pushes update in <100ms; polling has a 0-2s
  lag depending on when the new signal arrives relative to the next
  poll.
- **Server load** — one tab = 30 requests/min on `/v1/incidents` even
  if nothing changed.
- **Battery / mobile** — polling drains battery faster than an idle
  WebSocket.

Trade-off was deliberate: WebSocket adds backend state (the
connection registry), complicates Phase 6 chaos tests, and is on the
explicit bonus list (B2). Polling is the right default for a v1.

### Q: We hand-wrote Tailwind components instead of shadcn/ui. What
did we give up?
**A:** shadcn ships ~50 polished, accessible components (combobox,
dialog, dropdown menu, sheet, calendar) built on Radix primitives.
We have ~3 (Badge variants, an inline Field wrapper). When the
dashboard needs a combobox or a date picker, we'll have to either
hand-roll it or bring in shadcn after all.

We accepted that cost because the dlx hung and we needed to move.
Phase 7 polish can reintroduce shadcn for any specific component
that gets gnarly.

### Q: Why no auth on the dashboard?
**A:** Same PRD NG1: "Authentication and authorization. The dashboard
is open. In production this would sit behind SSO; out of scope here."
Real production: OAuth + JWT + a `Authorization: Bearer` middleware
on the API. Our `actor` field on transitions is whatever the dashboard
sends ("dashboard") because there's no authenticated user to derive
it from.

### Q: We didn't add a gRPC rate limiter. Is that a problem?
**A:** Theoretically a peer could blast the gRPC server hard enough to
overwhelm the pipeline. In practice the bounded channel still
back-pressures — gRPC peers see `Ack_ACK_STATUS_REJECTED_QUEUE_FULL`
when Submit fails. Phase 6 chaos tests will surface whether that's
actually sufficient at 10K/sec with a misbehaving client.

### Q: Why not regenerate the gRPC code on every build?
**A:** Generated code is checked in. The build doesn't need protoc
installed. Trade-off: a reviewer modifying the .proto must remember
to run `buf generate`. Worth it for the simpler build story.

---

## 6. Interview gotchas

> *Tight answers; expand from above if pressed.*

**1. "Walk me through what happens when a gRPC client sends a Signal."**
1. Client opens a streaming RPC on `localhost:9090`.
2. gRPC server reads one `Signal` proto message from the stream.
3. `handleOne` builds a `model.Signal`, runs `Validate()`. Failure → REJECTED_INVALID ack.
4. `pipeline.Submit(sig)` — non-blocking enqueue (same call HTTP makes). Full → REJECTED_QUEUE_FULL ack.
5. ACCEPTED ack sent on the stream.
6. Worker eventually picks up the signal, runs the debounce + fan-out + alerter — same code path as an HTTP signal.

**2. "Why bidi-stream over server-streaming or unary?"**
- Unary: per-signal handshake overhead unacceptable at 10K/sec.
- Server-streaming: client can only send one initial request; can't keep streaming new signals.
- Bidi: client streams Signals, server streams Acks. Matches the
  workload exactly.

**3. "Why is the proto's payload a `bytes` field?"**
Heterogeneous payloads with stable wire format. Struct would force a
JSON-tree marshal/unmarshal on both ends (cost) and create a mismatch
with the HTTP path (which is already `json.RawMessage`). Bytes is the
common denominator.

**4. "How do you handle a slow client in a gRPC stream?"**
HTTP/2 flow control. The server reads at its own pace from the
stream; if it stops reading, the client's `Send` eventually blocks
when the window fills. We never explicitly throttle; the pipeline's
queue-full → REJECTED ack is the rate signal the client should
respond to.

**5. "What's a Server Component vs Client Component?"**
Server: runs once on the server, output is sent as HTML, no hooks,
no event handlers. Client (`'use client'`): runs in the browser,
full React. Use Server by default; Client only when interactivity is
needed.

**6. "Why is the live feed `'use client'`?"**
Polling needs `setInterval` + `useState` + `useEffect`, all
client-only APIs. Server Components render once and freeze.

**7. "Walk me through what CORS is and why we need it."**
Browser security: JS on origin A can't read responses from origin B
unless B says "yes, A is allowed." The API at `localhost:8080`
returns `Access-Control-Allow-Origin: http://localhost:3000` on
every cross-origin response. For non-simple requests (anything with
a JSON body), the browser first sends an OPTIONS preflight; if that
gets a 2xx with the right headers, the real request proceeds.

**8. "Why is SQLSTATE 40001 not a real error?"**
Because 40001 means Postgres preserved SERIALIZABLE semantics by
aborting one of two conflicting transactions. The system is working
*correctly*. The right response is "client retries the same request,"
which is exactly what 409 Conflict means in HTTP. 500 would imply
broken backend.

**9. "Why is the workflow tx SERIALIZABLE but the processor's UPDATE
isn't?"**
Different contention profiles. Workflow transitions are
low-frequency, multi-statement, and need phantom-read protection on
the audit table → SERIALIZABLE pays off. The processor's
IncrementSignalCount is high-frequency, single-statement, and
row-locks already serialize at READ COMMITTED. SERIALIZABLE there
would generate 40001 churn for no correctness win.

**10. "Walk me through what happens when the user clicks 'Submit RCA'."**
1. Client validates fields locally (mirror of `model.RCA.Validate`).
   Bad → render field errors inline, no request.
2. `POST /v1/incidents/:id/rca` with the RCA body.
3. Server: `workflow.Engine.CloseWithRCA` opens a SERIALIZABLE tx,
   locks the work_item row, runs `ResolvedState.CanTransitionTo(ClosedState, ctx)`.
4. If RCA invalid: `IncompleteRCAError` → 422 with field array.
5. If valid: `ClosedState.OnEnter` computes MTTR, stamps it on the
   WI. Server inserts rca row, updates work_items, inserts
   state_transition. Commits.
6. Frontend gets 201, redirects to `/incidents/:id`.
7. The detail page re-fetches; sees status=CLOSED, mttr_seconds=N, rca panel populated.

**11. "What if Postgres returns 40001 mid-RCA?"**
The handler returns 409. The frontend's APIError carries that
status. We could add a retry loop client-side; v1 just surfaces the
409 to the user (the dashboard form keeps the data on screen, so
they can click submit again).

**12. "Why is the generated proto code checked into git?"**
Reproducibility. A reviewer can `go build ./...` without installing
protoc + buf + the protoc-gen-* plugins. Cost: editing the .proto
requires re-running buf generate and committing both. Worth it.

---

## 7. Things you should be able to do after Phase 5

- [ ] Write a tiny gRPC client (~30 lines) that streams 10 signals to
      `:9090` and prints the acks. (See `scripts/grpc-client.go`.)
- [ ] Explain why `pipeline.Submitter` is a tiny interface defined in
      `internal/ingest/handler.go` and consumed by both the HTTP and
      gRPC handlers.
- [ ] Open the dashboard, follow PRD Goal G3 end-to-end through the
      UI (no curl): see an OPEN incident, advance it, submit a
      complete RCA, see the CLOSED status + MTTR.
- [ ] Edit `signals.proto` to add a new optional field (e.g.
      `tenant_id`). Run `buf generate`. Walk it through the gRPC
      server, the processor, and Mongo. What changes?
- [ ] Explain to a senior engineer why SQLSTATE 40001 → 409, not 500.
      Bonus: write the client-side retry loop for the RCA form.
- [ ] Modify the dashboard's polling interval to 5s via env var.
      What's the minimum diff? (Hint: `NEXT_PUBLIC_POLL_INTERVAL_MS`.)
- [ ] Block CORS at the proxy level (or hand-set
      `VELLUM_CORS_ORIGINS=`) and watch the browser console fail to
      load `/v1/incidents`. Explain the preflight failure mode.

If those feel comfortable, Phase 5 is solid in your head.
