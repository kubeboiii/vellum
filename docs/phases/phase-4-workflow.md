# Phase 4 — Workflow Engine

> **Day:** 4 of 7
> **Depends on:** Phase 3 (Debounce & Persistence Fan-out) merged on main.
> **References:** `00-master-prd.md` §4.4, §4.5, §4.6, §5.3; `01-architecture.md` §7, §9.
> **Out of scope:** gRPC ingestion (Phase 5), dashboard UI (Phase 5), failure simulator (Phase 6).

---

## 1. Goal in one sentence

Make a Work Item *progress*: implement the State pattern for the
`OPEN → INVESTIGATING → RESOLVED → CLOSED` lifecycle with mandatory
RCA on close, wrap each transition in a SERIALIZABLE Postgres
transaction (CLAUDE.md design rule 2), implement the Strategy pattern
for alerters (PagerDutyStub / SlackWebhook / Console), and expose two
new endpoints:

- `PATCH /v1/incidents/:id/state` — advance one step
- `POST /v1/incidents/:id/rca` — submit RCA + close in one atomic op

---

## 2. The four critical rules — which apply this phase

| Rule | This phase? | How |
|---|---|---|
| 1. Ingestion never blocks on persistence | ✅ existing | Unchanged. Alerter dispatch runs in its own goroutine off the worker hot path (FR-6.4). |
| 2. State transitions are transactional | ✅ NEW | `workflow.Engine.Transition` runs BEGIN → SELECT FOR UPDATE → state-pattern check → UPDATE work_items → INSERT state_transitions → COMMIT. SERIALIZABLE isolation. |
| 3. RCA required for CLOSED | ✅ NEW | `ResolvedState.CanTransitionTo(ClosedState, ctx)` is the single enforcement point. `ErrMissingRCA` and `ErrIncompleteRCA` map to 422 in the handler. |
| 4. Debounce is atomic via Lua | ✅ existing | Untouched. |

---

## 3. Scope — what we build

### 3.1 New packages

| Package | Responsibility |
|---|---|
| `internal/workflow` | State interface + 4 concrete states (Open/Investigating/Resolved/Closed) + `Engine` orchestrator that runs transitions in a Postgres transaction. |
| `internal/alert` | `Alerter` interface (Strategy) + 3 implementations (PagerDutyStub, SlackWebhook, Console) + `Registry` that picks one by severity. |

### 3.2 Extensions

- `internal/model`: add `RCA` struct + `RootCauseCategory` enum + `RCA.Validate()`.
- `internal/persist/pg`: add `RCARepository` (Insert/GetByWorkItemID) and extend
  `WorkItemRepository` with the transactional `Transition` method that the
  workflow engine calls.
- `internal/processor`: after a CREATED Postgres write, dispatch the alert
  asynchronously via the alert registry (FR-6.4).
- `internal/api`: new package — Gin handlers for the two new endpoints +
  `GET /v1/incidents` (live feed) and `GET /v1/incidents/:id` for Phase 5
  prep. (Phase 5 builds the frontend; Phase 4 puts the API in place.)

### 3.3 New SQL migration

`004_rca.up.sql` / `004_rca.down.sql`: creates the `rca` table per
`00-master-prd.md` §4.5:

```sql
CREATE TABLE rca (
    id                   uuid PRIMARY KEY,
    work_item_id         uuid NOT NULL UNIQUE
                              REFERENCES work_items(id) ON DELETE CASCADE,
    incident_start       timestamptz NOT NULL,
    incident_end         timestamptz NOT NULL,
    root_cause_category  text NOT NULL,
    fix_applied          text NOT NULL,
    prevention_steps     text NOT NULL,
    submitted_by         text NOT NULL DEFAULT 'system',
    created_at           timestamptz NOT NULL DEFAULT now(),

    CONSTRAINT rca_category_chk
        CHECK (root_cause_category IN ('CODE_DEFECT','INFRASTRUCTURE',
            'CONFIG_CHANGE','EXTERNAL_DEPENDENCY','CAPACITY','HUMAN_ERROR','OTHER')),
    CONSTRAINT rca_fix_min_length
        CHECK (char_length(fix_applied) >= 20),
    CONSTRAINT rca_prevention_min_length
        CHECK (char_length(prevention_steps) >= 20),
    CONSTRAINT rca_time_order
        CHECK (incident_end >= incident_start)
);
```

App-level `Validate()` in `internal/model` enforces the same rules
client-side. The DB constraints are belt-and-braces — a `psql` user
bypassing the app can't poison the schema.

### 3.4 Endpoints

| Method | Path | Body | Codes |
|---|---|---|---|
| `PATCH` | `/v1/incidents/:id/state` | `{"to":"INVESTIGATING","reason":"...","actor":"sre@example.com"}` | 200, 400 (bad enum), 404 (no such WI), 409 (illegal transition / concurrent update), 422 (rule violation) |
| `POST` | `/v1/incidents/:id/rca` | RCA fields per §4.5 | 201 (RCA created + WI CLOSED), 404, 422 (missing/incomplete RCA) |
| `GET` | `/v1/incidents` | — | 200; list of non-CLOSED Work Items sorted by severity then `last_signal_ts` |
| `GET` | `/v1/incidents/:id` | — | 200 (WI + RCA if present), 404 |

`POST /rca` is a compound operation: it inserts the RCA row AND
transitions the WI to CLOSED in **one** Postgres transaction. If the
RCA insert succeeds but the close fails (or vice versa), both roll
back. This is exactly what the State pattern's "single enforcement
point" buys us — the rule "CLOSED requires RCA" lives in one place
and the transaction wraps it.

### 3.5 Alerting (Strategy pattern)

Three concrete alerters per FR-6.2:

| Alerter | Triggers on | Behaviour |
|---|---|---|
| `PagerDutyStub` | severity = `P0` | Log a structured JSON line to stdout (simulating the PD Events API payload) |
| `SlackWebhook` | severity = `P1`, `P2` | HTTP POST to `SLACK_WEBHOOK_URL` if set; falls back to Console if unset |
| `Console` | severity = `P3` and fallback | Log a one-line alert summary |

The `Registry.ForWorkItem(wi)` returns the right `Alerter` based on
ordered match rules. Adding a new alerter is one file + one Rule entry
in the registry constructor.

### 3.6 Config (env, added this phase)

| Var | Default | Why |
|---|---|---|
| `SLACK_WEBHOOK_URL` | (empty) | If set, P1/P2 alerts POST to this URL. Else Console. |
| `VELLUM_ALERTER_TIMEOUT` | `5s` | Per-dispatch timeout (FR-6.4) so a slow webhook can't pile up goroutines |

---

## 4. Implementation order

1. **Migration 004**: write `rca.up.sql` + `.down.sql`; run `migrate up`;
   verify with `psql`.
2. **Model**: `model/rca.go` (RCA + `RootCauseCategory` + `Validate`)
   and `model/state.go` (just the `Status` constants already exist;
   nothing new needed here).
3. **Workflow**: `internal/workflow/state.go` (the interface + 4 state
   types + sentinel errors), `internal/workflow/engine.go` (the
   transactional `Transition` method). Tests with a stubbed
   `WorkItemRepository` fake for the pure state logic; testcontainers
   for the full transactional path.
4. **Alerter**: `internal/alert/alerter.go` (interface + Registry),
   `internal/alert/console.go`, `internal/alert/slack.go`,
   `internal/alert/pagerduty.go`. Tests with `httptest.Server` for
   Slack.
5. **Persistence**: extend `pg.WorkItemRepository` with
   `TransitionWithRCA` (the compound RCA+close op) and a `Transition`
   that runs the State pattern check inside a SERIALIZABLE tx. Add
   `pg.RCARepository`.
6. **Wire alerters into processor**: when action == CREATED, after the
   Postgres write succeeds, fire `alerter.ForWorkItem(wi).Dispatch(...)`
   in a `go` block with a 5s timeout context.
7. **API**: `internal/api/` package with the four new handlers.
   `cmd/vellum/main.go` wires them onto Gin under `/v1`.
8. **Tests**: per-package unit + a few end-to-end happy/sad paths
   against testcontainers.

---

## 5. Acceptance criteria

- [ ] `migrate up` (after migration 004) creates the `rca` table; `down` removes it; `up` again is idempotent.
- [ ] `go test -race ./...` clean across all packages, including the
      *concurrency-focused* test from R2 in 00-master-prd: two
      goroutines try to PATCH the same WI to CLOSED at the same time
      → exactly one wins, the other gets 409.
- [ ] **PRD §7 Goal G3 end-to-end:**
   1. `PATCH /v1/incidents/:id/state {"to":"INVESTIGATING"}` on an OPEN incident → 200, audit row written.
   2. Then `PATCH ... {"to":"RESOLVED"}` → 200.
   3. Then `PATCH ... {"to":"CLOSED"}` (no RCA) → 422 with `error: missing RCA`.
   4. Then `POST /v1/incidents/:id/rca` with `fix_applied: "short"` → 422 with field errors.
   5. Then `POST .../rca` with a complete RCA → 201; response includes the WI with `status=CLOSED` and `mttr_seconds` populated.
- [ ] Backward transitions are rejected with 409 (e.g., RESOLVED → OPEN).
- [ ] Skip transitions are rejected with 409 (e.g., OPEN → CLOSED).
- [ ] When a CREATED P0 work_item lands in Postgres, the PagerDutyStub
      logs a structured payload within 100ms (verified by tailing
      stdout during the storm script).
- [ ] When `SLACK_WEBHOOK_URL` is set, a P1 work_item triggers an HTTP
      POST to that URL (verified with a local `httptest.Server` in the
      test); when unset, the Console alerter is used.
- [ ] A failing Slack webhook (5s timeout exceeded) doesn't crash the
      processor, doesn't dead-letter the signal, doesn't block
      subsequent workers. Logged and moved on.
- [ ] All four state-pattern transitions and all rejections covered by
      unit tests (rubric requires ≥60% coverage on core packages).

---

## 6. Non-goals (do **not** add this phase)

- gRPC ingestion (Phase 5).
- Any frontend/UI work — the new GET endpoints exist for Phase 5 to
  consume but no React/Next code is written here.
- Real PagerDuty / Datadog / Jira integration (PRD NG3).
- Auth on the new PATCH/POST endpoints (PRD NG1).
- Severity escalation, manual incident merging, comments on RCAs
  (all explicitly bonus features, PRD §11).

---

## 7. Learning notes (whiteboard check)

After Phase 4, be able to explain unaided:

1. What does the State design pattern give us that a `switch status` would not? (Reference: 01-arch §7.2.)
2. Why SERIALIZABLE isolation and not READ COMMITTED for the transition transaction? (Reference: 01-arch §12.)
3. What does `SELECT FOR UPDATE` actually do at the row level? Why is it necessary even with SERIALIZABLE?
4. Walk through what happens when two `PATCH .../state` requests for the same WI hit the server simultaneously — which one wins, what does the loser get back?
5. Where does the rule "CLOSED requires RCA" live? Why exactly one place?
6. Why is alert dispatch async (`go alerter.Dispatch(...)`) rather than retryable+dead-lettered like the sink writes?
7. What does MTTR mean and *when* is it computed? (When entering `ClosedState`. Stored on the WI row.)
8. Why does the RCA POST endpoint do *two* things (insert RCA + close WI) atomically? What goes wrong if we don't?

If any of these is fuzzy, re-read the cited section before writing code.
