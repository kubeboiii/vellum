# Phase 5 — gRPC + Frontend

> **Day:** 5 of 7
> **Depends on:** Phase 4 (Workflow Engine) merged on main.
> **References:** `00-master-prd.md` §4.1, §4.7, §6, §7; `01-architecture.md` §3.1, §4, §8.
> **Out of scope:** Failure simulator (Phase 6), polish/docs (Phase 7), WebSocket push (PRD B2 bonus).

---

## 1. Goal in one sentence

Add the **second ingestion protocol** (gRPC server-streaming on `:9090`,
sharing the existing in-process pipeline) and the **Next.js dashboard**
(live feed, incident detail, RCA form) so an SRE can actually triage
incidents in a browser.

After Phase 5: `docker compose up && go run ./cmd/vellum && pnpm dev` →
a working triage UI at `localhost:3000` reading from the same backend
that's accepting signals on HTTP and gRPC.

---

## 2. The four critical rules — which apply this phase

| Rule | This phase? | How |
|---|---|---|
| 1. Ingestion never blocks on persistence | ✅ NEW (gRPC) | The gRPC `IngestSignals` handler does the SAME non-blocking enqueue as the HTTP handler. One pipeline, two protocols (FR-1.3). |
| 2. State transitions are transactional | ✅ existing | Untouched. The dashboard's PATCH/POST endpoints route through the workflow engine from Phase 4. |
| 3. RCA required for CLOSED | ✅ existing | Untouched. The RCA form submits to `POST /v1/incidents/:id/rca` which uses `workflow.Engine.CloseWithRCA`. |
| 4. Debounce is atomic via Lua | ✅ existing | Untouched. |

---

## 3. Scope — what we build

### 3.1 Backend changes

| File / Package | Responsibility |
|---|---|
| `backend/proto/signals.proto` | Service + message definitions: `SignalService.IngestSignals(stream Signal) returns (stream Ack)`. Fields match `model.Signal` (FR-2.2). |
| `backend/proto/signals.pb.go`, `signals_grpc.pb.go` | Generated. Checked in so reviewers don't need protoc. |
| `backend/buf.yaml`, `backend/buf.gen.yaml` | Buf config: lint rules + `buf generate` plugin pipeline. |
| `internal/ingest/grpc.go` | gRPC `SignalServiceServer` implementation. Same `pipeline.Submitter` contract as the HTTP handler. |
| `internal/api/signals.go` | NEW handler: `GET /v1/incidents/:id/signals` returns paginated raw signals from Mongo (powers Phase 5 detail page, FR-7.2). |
| `internal/persist/mongo/signal_repo.go` | Add `ListByWorkItem(ctx, wiID, page, perPage)` reader. |
| `cmd/vellum/main.go` | Start the gRPC listener on `:9090` alongside the HTTP listener. Both shutdown gracefully. Add CORS middleware so localhost:3000 can call the API in dev. |

**No new Go runtime deps** other than `google.golang.org/grpc` and
`google.golang.org/protobuf` (already pulled by the proto stubs).

### 3.2 Frontend changes (Next.js 14, App Router)

| File | Responsibility |
|---|---|
| `frontend/lib/api.ts` | Typed fetch client. One module exporting `listIncidents`, `getIncident`, `listSignals`, `patchState`, `postRCA`. Reads `NEXT_PUBLIC_API_BASE` (default `http://localhost:8080`). |
| `frontend/lib/types.ts` | TypeScript mirrors of `WorkItem`, `RCA`, `Signal`, `Severity`. Hand-written; small enough not to need codegen. |
| `frontend/app/page.tsx` | Live feed (Client Component, polls every 2s). Renders a table sorted by severity then last_signal_ts. |
| `frontend/app/incidents/[id]/page.tsx` | Detail page (Server Component for initial fetch + Client Component for state-transition controls). Shows the work_item, the linked signals (paginated), the state-transition history, and RCA (if present). |
| `frontend/app/incidents/[id]/rca/page.tsx` | RCA form. Server-side and client-side validation mirroring `model.RCA.Validate`. Submits to `POST /v1/incidents/:id/rca`. |
| `frontend/components/ui/*` | shadcn/ui components (button, card, badge, input, textarea, select). Initialized via `pnpm dlx shadcn@latest init`. |
| `frontend/components/SeverityBadge.tsx`, etc. | Tiny composed components built on shadcn. |

### 3.3 New backend endpoint

| Method | Path | Body | Codes |
|---|---|---|---|
| `GET` | `/v1/incidents/:id/signals?page=1&per_page=50` | — | 200 with `{items, page, per_page, total}`. Reads from Mongo via `signal_repo.ListByWorkItem`. |

### 3.4 gRPC contract

```proto
syntax = "proto3";
package ims.v1;
option go_package = "github.com/kubeboiii/ims/proto;imspb";

service SignalService {
  // Client streams signals; server acks each one with the assigned
  // signal_id (server may mint UUIDs for clients that don't supply
  // one). Server-side close ends the RPC.
  rpc IngestSignals(stream Signal) returns (stream Ack);
}

message Signal {
  string signal_id      = 1;  // optional; server fills in if empty
  string component_id   = 2;
  string component_type = 3;
  string severity       = 4;
  google.protobuf.Timestamp timestamp = 5;
  string source         = 6;
  bytes  payload        = 7;  // raw JSON
}

message Ack {
  string signal_id = 1;
  enum Status { ACCEPTED = 0; REJECTED_QUEUE_FULL = 1; REJECTED_INVALID = 2; }
  Status status = 2;
  string error  = 3; // populated when status != ACCEPTED
}
```

Bidirectional streaming, mirroring HTTP's "fire many, get many 202s"
shape. Backpressure mechanism is identical: non-blocking `Submit` to the
shared pipeline, return `REJECTED_QUEUE_FULL` when the channel is full.

### 3.5 Config (env, added this phase)

| Var | Default | Why |
|---|---|---|
| `VELLUM_GRPC_ADDR` | `:9090` | gRPC bind address (01-arch §3.1) |
| `VELLUM_CORS_ORIGINS` | `http://localhost:3000` | Comma-separated allowed origins for the dashboard. Dev-only — production would gate via a proxy. |
| `NEXT_PUBLIC_API_BASE` | `http://localhost:8080` | Frontend → backend URL. `NEXT_PUBLIC_` prefix exposes it to the browser bundle. |

---

## 4. Implementation order

1. `backend/proto/signals.proto` + `backend/buf.yaml` + `backend/buf.gen.yaml`. Run `buf generate` and check the output into `backend/proto/`.
2. `internal/ingest/grpc.go` implementing `SignalServiceServer.IngestSignals`. Unit test with `bufconn` (in-memory gRPC) so we don't need a real port.
3. `cmd/vellum/main.go`: start the gRPC server alongside HTTP. Add CORS middleware (`gin-contrib/cors`).
4. `internal/persist/mongo/signal_repo.go`: `ListByWorkItem(ctx, wiID, page, perPage)`. Testcontainers test.
5. `internal/api/signals.go`: `GET /v1/incidents/:id/signals` handler.
6. `frontend`: shadcn init + add `button card badge input textarea select table`. Hand-roll the typed API client.
7. `frontend/app/page.tsx`: live feed with 2s polling.
8. `frontend/app/incidents/[id]/page.tsx`: detail + state controls + signals list.
9. `frontend/app/incidents/[id]/rca/page.tsx`: RCA form with field-level validation.
10. End-to-end smoke test: storm script + open dashboard, click through.

---

## 5. Acceptance criteria

- [ ] `buf generate` produces clean Go code with no warnings; the
      generated files compile.
- [ ] `go test -race ./...` clean across all packages, including
      the new gRPC test.
- [ ] `go vet ./...`, `gofmt -l .` clean.
- [ ] **gRPC end-to-end:** a Go client (test or scripts/grpc-client.go)
      streams 10 signals to `:9090`, receives 10 ACCEPTED acks within
      100ms total, and the signals appear in Mongo + Postgres via the
      same pipeline as HTTP.
- [ ] **Dashboard end-to-end** (via browser, after `pnpm dev`):
   1. `GET localhost:3000/` shows the live feed; with a few signals
      sent, the table has rows. Updates within 2s of new signals.
   2. Click an incident → detail page renders work_item, raw signals
      (paginated), state-transition timeline.
   3. Use the state buttons: OPEN→INVESTIGATING→RESOLVED. The page
      updates the visible status.
   4. Navigate to RCA form, submit incomplete RCA → field errors
      shown inline. Submit complete RCA → success, redirect back to
      detail, status is now CLOSED with MTTR visible.
- [ ] All three frontend pages render without console errors.
      `pnpm build` succeeds.
- [ ] `docker compose up` + the README run sequence brings the entire
      system up. No additional manual steps for the dashboard beyond
      `pnpm dev`.

---

## 6. Non-goals (do **not** add this phase)

- WebSocket push for the live feed (PRD B2 bonus).
- Real-time charts / graphs.
- Light/dark theming, mobile-responsive design.
- Auth or per-user view.
- gRPC reflection beyond what `grpc-go` provides by default.
- Failure simulator (Phase 6).

---

## 7. Learning notes (whiteboard check)

After Phase 5, be able to explain unaided:

1. Why does the gRPC server share the in-process pipeline with HTTP, instead of running its own? (FR-1.3 + 01-arch §3.1.)
2. What's a Server Component vs Client Component in Next.js App Router, and when do you use which?
3. Why is the live feed Client-side polling, not SSR with revalidate?
4. What does `'use client'` actually do at runtime?
5. Why is `VELLUM_CORS_ORIGINS` only an issue for local dev?
6. Walk through what `bufconn` is and why we use it in the gRPC test.
7. How does the dashboard handle a 422 from `POST /rca`? Where does the field-error mapping live (client side mirror of `model.RCA.Validate`)?
8. Why is the proto's `payload` a `bytes` field, not a `google.protobuf.Struct`?

If any of these is fuzzy, re-read the cited section before writing code.
