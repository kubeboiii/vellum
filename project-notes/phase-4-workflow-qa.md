# Phase 4 — Workflow Engine: Q&A Study Guide

> **Phase scope:** State pattern (Open/Investigating/Resolved/Closed)
> with mandatory RCA on close, Strategy pattern for alerters
> (PagerDuty / Slack / Console), MTTR computation in
> `ClosedState.OnEnter`, and SERIALIZABLE-tx state transitions. New
> endpoints: `PATCH /state` and `POST /rca`.
>
> **Acceptance result:** PRD G3 verified end-to-end (no-RCA close → 422,
> incomplete RCA → 422 with field details, complete RCA → 201 with MTTR
> populated). Concurrent-close test: exactly one of two parallel
> CLOSE requests succeeds.

---

## 1. What we built (one paragraph)

Until Phase 4 every Work Item sat at status=OPEN forever. Now: an
on-call SRE can `PATCH` it through INVESTIGATING → RESOLVED, then
submit an RCA via `POST` to drive it to CLOSED. The State pattern
encodes which transitions are legal (no backwards, no skipping,
RCA required to close). MTTR is computed automatically when entering
CLOSED. Every transition writes an audit row in one Postgres
transaction with row-level locking, so two concurrent closes for the
same incident can't both succeed. The Strategy pattern routes new
incidents to the right alerter (PagerDuty stub for P0, Slack for
P1/P2 with a console fallback, Console for everything else).

---

## 2. The fundamentals

### Q: What's the "State design pattern"?
**A:** Each state is its own type with its own behaviour. Instead of:

```go
func transition(wi *WorkItem, to Status) error {
    switch wi.Status {
    case OPEN:
        if to == INVESTIGATING { ... } else { return error }
    case RESOLVED:
        if to == CLOSED { if rca == nil { return error } ... }
    // ...
    }
}
```

you write:

```go
type State interface {
    CanTransitionTo(next State, ctx ...) error
    OnEnter(ctx, wi) error
}
type OpenState struct{}
func (OpenState) CanTransitionTo(next State, _ ...) error { ... }
```

The state's *rules* and *side effects* live with the state type.
Adding a new state = one new file. Reviewer looking for a rule
finds it in O(1).

### Q: What's the "Strategy design pattern"?
**A:** A family of algorithms, each with the same interface, chosen
at runtime. Our `Alerter` interface has two methods (`Name`, `Dispatch`);
three implementations (`PagerDutyStub`, `SlackAlerter`, `ConsoleAlerter`).
A `Registry` picks one per Work Item based on severity rules. Adding
a fourth (MS Teams, OpsGenie) is one struct + one rule. **Zero
changes to the processor.**

### Q: What's "transactional integrity" mean here?
**A:** A multi-step operation either fully happens or fully doesn't.
Our state transition is: (a) UPDATE work_items.status, (b) INSERT a
row into state_transitions. If the server crashes between (a) and (b)
we end up with an "orphan transition" — the WI moved, no audit
record. SERIALIZABLE transaction = both or neither.

### Q: What's "SERIALIZABLE" isolation, vs READ COMMITTED?
**A:** Postgres isolation levels:
- **READ COMMITTED** (default): each statement sees the DB as of when
  it started. Two concurrent transactions can each read the same row
  and both write — last write wins.
- **SERIALIZABLE**: as if transactions ran one at a time. Postgres
  may abort one transaction with `serialization_failure` if it
  detects conflict. Strongest guarantee, slowest in pure-throughput
  benchmarks but trivial in low-contention workloads (like ours,
  where transitions are human-driven).

### Q: What's `SELECT ... FOR UPDATE`?
**A:** A row-level lock. The first transaction to issue `SELECT ...
FOR UPDATE` on a row holds it; a second transaction's `SELECT ...
FOR UPDATE` *blocks* until the first commits or rolls back. We use
this to serialize concurrent transitions on the same WI: both
goroutines reach the SELECT, one wins, the other waits, then
re-evaluates the state (and gets ErrInvalidTransition because the
WI has already advanced).

### Q: What's MTTR and when do we compute it?
**A:** Mean Time To Repair = `incident_end - incident_start`. We
compute it in `ClosedState.OnEnter` — the single place where a WI
transitions into CLOSED. That keeps the logic in one place; no
caller has to remember to call `computeMTTR()` before close.

### Q: What's an "RCA"?
**A:** Root Cause Analysis. A structured form filled in after the
incident: when did it start, when did it end, what category of
failure was it, what fixed it, what will prevent recurrence. We
require it for CLOSED so teams can't paper over incidents without
documenting them.

### Q: What's "fire-and-forget" mean?
**A:** Dispatch the work in a goroutine and don't wait for the
result. Caller continues immediately. Used for alerts (FR-6.4): a
slow Slack webhook must not block the worker that's processing the
next signal. Errors are logged but not propagated.

### Q: What's the difference between `errors.Is` and `errors.As`?
**A:**
- `errors.Is(err, target)` — true if `err` *wraps* `target` somewhere
  in its chain. Use for sentinel comparisons like `errors.Is(err,
  ErrMissingRCA)`.
- `errors.As(err, &target)` — unwraps `err` until it finds something
  that's of the target's type, stores it. Use when you want to pull
  out a struct error with extra fields (`*IncompleteRCAError` has a
  list of field errors we want to surface).

---

## 3. The tech we used

### Q: What's `pgx.TxOptions{IsoLevel: pgx.Serializable}`?
**A:** Tells pgx to run `BEGIN ISOLATION LEVEL SERIALIZABLE` instead
of plain `BEGIN`. Goes on the same transaction the engine uses for
all the writes.

### Q: What's an `httptest.Server`?
**A:** stdlib helper. `httptest.NewServer(handlerFunc)` starts a real
HTTP server on a random port and returns its URL. We use it to test
`SlackAlerter` — point the alerter at `srv.URL` and observe what it
POSTs, without depending on a real Slack workspace.

### Q: What's `model.FieldError` and why a slice of them?
**A:** A `{field, error}` pair. When `RCA.Validate()` finds problems,
it returns ALL of them as `[]FieldError` so the user can fix
everything in one round-trip. Returning the first error and stopping
would make the user submit-fix-submit-fix iteratively.

### Q: What's `gin.RouterGroup` vs `gin.Engine`?
**A:** Engine is the top-level router. `r.Group("/v1")` returns a
RouterGroup that prefixes every route registered through it with
`/v1`. So `v1.POST("/incidents/:id/rca", ...)` mounts at
`/v1/incidents/:id/rca`. Lets us version the API at one place.

### Q: What's a "compound endpoint"?
**A:** One HTTP request that does logically multiple things atomically.
`POST /rca` inserts the RCA row AND transitions the WI to CLOSED
inside one Postgres transaction. Easier client contract (one call,
one response) and stronger guarantees (either both rows or neither).

---

## 4. The design decisions

### Q: Why is `ResolvedState.CanTransitionTo` the only place the RCA
rule lives?
**A:** Single Source of Truth. If we ALSO checked "is RCA present?"
in the API handler, two-place rules drift: someone tightens the
validation in the State pattern, the handler keeps the old form,
bugs follow. The State pattern is the right place because *every*
path to CLOSE (current PATCH endpoint, future CLI tool, future gRPC
endpoint) routes through it.

### Q: Why is the State pattern interface only 3 methods?
**A:** Smaller interfaces = fewer mock methods, fewer breaking
changes when we add a method, clearer mental model. `Name()`,
`CanTransitionTo()`, `OnEnter()` are exactly what we need. We don't
add `OnExit()` because nothing currently needs it; if Phase 7 adds
"cleanup when leaving INVESTIGATING," we add it then.

### Q: Why is `ClosedState.CanTransitionTo` always an error?
**A:** CLOSED is terminal in our state machine. No "REOPEN" in v1.
Defining the method to always return `ErrInvalidTransition` makes
the rule explicit in the type — a reader can see at a glance "you
can't transition out of CLOSED."

### Q: Why does the engine wrap rollback in `defer`, even on success?
**A:** Pgx makes Rollback after Commit a no-op. So `defer
tx.Rollback()` is a safety net: if any return path between BeginTx
and Commit forgets to clean up, the deferred Rollback fires. Cleaner
than three explicit Rollback calls across error branches.

### Q: Why aren't alert errors retried?
**A:** Alerts are not source of truth. Retrying a slow Slack call
means the worker waits — that's exactly what FR-6.4 prohibits.
Retrying with backoff in a separate goroutine means stacking dozens
of pending alerts on a flapping webhook. We log + skip; that's the
right policy for "FYI" messages.

### Q: Why does the registry's `ForWorkItem` walk rules in order?
**A:** Order matters: a P0 incident matches "P0 → PagerDuty" before
it matches "P0,P1,P2 → Slack" (if both existed). Ordered match also
makes "default to console" trivial — put the catch-all rule last,
or rely on the `console` fallback when nothing matches.

### Q: Why is `IncompleteRCAError` a struct (not a sentinel error)?
**A:** Because we need to carry data (the list of field errors). A
sentinel is just `errors.New("..."`); it has no payload. The struct
implements `Error()` for normal display AND `Unwrap()` so
`errors.Is(err, ErrIncompleteRCA)` works for callers that just want
the kind, not the details. `errors.As(err, &target)` lets the
handler pull the field list out.

### Q: Why does `POST /rca` return 201, not 200?
**A:** 201 Created is the right code for "I made a new resource."
The new resource is the RCA. Closing the WI is a side effect. 200
would technically work; 201 is more precise and aligns with REST
conventions.

### Q: Why is the API package separate from `internal/ingest`?
**A:** Different lifecycle, different deps. `ingest` is the hot path:
non-blocking enqueue, rate-limited, sub-millisecond. `api` is
human-driven: pulls in `workflow.Engine`, `pg.WorkItemRepository`,
`pg.RCARepository`. Putting them together would force `ingest` to
transitively depend on `workflow`, which it shouldn't.

---

## 5. Tradeoffs

### Q: SERIALIZABLE is the strongest isolation level. Is it overkill?
**A:** For high-contention workloads, yes — it would force serialization
failures + app-side retries. For our workload (transitions are
human-driven, low frequency), the contention is near zero, so the
strong guarantee is essentially free. We pick SERIALIZABLE because
it's the most defensible choice in an interview ("why didn't you go
strongest? because it costs nothing here"), not because we measured
a perf win.

### Q: Why not use Temporal/Cadence for workflow orchestration?
**A:** Temporal is brilliant for *long-running* workflows with human
input + retries spanning days (order fulfillment, document review).
Our workflow is a 4-state state machine where each transition is one
HTTP request. The overhead of running a Temporal cluster + writing
workflow code + onboarding the team would dwarf the actual work.

### Q: We're not enforcing auth on PATCH/POST endpoints. Why?
**A:** PRD NG1: "Authentication and authorization. The dashboard is
open. In production this would sit behind SSO; out of scope here."
For a 7-day demo with no real PII, no real responders, no real
incidents, auth would be theater. In a real system you'd put OIDC in
front of every PATCH/POST.

### Q: We're trusting the `actor` field from the request body. Anyone
can claim to be anyone. Why?
**A:** Same NG1 answer. With real auth, `actor` would come from the
JWT in the Authorization header, not the request body. v1 takes
whatever the client sends; the audit trail accurately records who
*claimed* to do the transition.

### Q: A failing Slack webhook silently drops the alert. Should we
persist failed alerts?
**A:** Could. Dead-letter pattern. But: alerts are FYI; if PagerDuty
was down for an hour and we replayed all 50 stuck pages an hour
later, we'd cause more confusion than a missed page. Real production
might log to a "missed_alerts" Mongo collection for operator review;
v1 logs to stderr and moves on.

---

## 6. Interview gotchas

> *Tight answers; expand from above if pressed.*

**1. "Walk me through what happens when I PATCH a work_item to CLOSED."**
1. Handler parses path id + JSON body.
2. `workflow.Engine.Transition(ctx, id, ClosedState{}, tctx)` is called.
3. Engine opens SERIALIZABLE pgx transaction.
4. `SELECT ... FROM work_items WHERE id = $1 FOR UPDATE` — locks the row.
5. Construct current State from row.status: probably `ResolvedState`.
6. `current.CanTransitionTo(ClosedState{}, tctx)` — runs `ResolvedState.CanTransitionTo`, which checks `tctx.RCA != nil` and `tctx.RCA.Validate() empty`. Without RCA: returns `ErrMissingRCA`. We rollback, return 422.

For the `POST /rca` path, the same flow but with `tctx.RCA` populated,
plus an `INSERT INTO rca` inside the same tx.

**2. "Why is the RCA rule enforced in CanTransitionTo, not in the
HTTP handler?"**
Single Source of Truth + future-proofing. If we add a CLI tool that
closes WIs without going through HTTP, it routes through the engine
→ State pattern → same rule. If we'd put the check in the handler,
the CLI tool would have to re-implement it, and the two
implementations would drift over time.

**3. "What happens if two operators try to close the same incident
at the same second?"**
Both requests hit `PatchState`. Both open SERIALIZABLE transactions.
Both attempt `SELECT FOR UPDATE`. One wins the row lock; that
transaction proceeds, commits, transitions WI to CLOSED. The loser
blocks on the lock; once unblocked it re-runs `CanTransitionTo` on a
row that's now CLOSED — `ClosedState.CanTransitionTo` always errors,
so the engine returns `ErrInvalidTransition` → handler returns 409.
Verified by `TestEngine_ConcurrentClose_ExactlyOneWins`.

**4. "Why is MTTR computed in `ClosedState.OnEnter` and not in the
handler?"**
Same reason: single source of truth. Any path that closes a WI runs
OnEnter. If we ever add a "bulk close stale incidents" admin tool,
it gets MTTR for free.

**5. "How is the alerter wired in?"**
At startup, main.go builds an `alert.Registry` with the three
alerters + match rules. It passes the registry to `processor.New`
as the `AlerterPicker` argument. When the processor's debounce
returns action=CREATED, after the Postgres write succeeds it calls
`registry.ForWorkItem(wi).Dispatch(ctx, wi)` — in a new goroutine,
with a fresh 5-second-deadline ctx. Errors are logged. The worker
returns to draining the next signal immediately.

**6. "Why is `IncompleteRCAError` a custom type instead of using
sentinel + extra fields somewhere else?"**
We want callers to do both:
- `errors.Is(err, ErrIncompleteRCA)` for the "is this the
  incomplete-RCA case?" check.
- `errors.As(err, &incomplete)` to pull out the field-level details
  to surface in the 422 response body.

A custom type that implements `Error() + Unwrap()` lets both work.
A sentinel + a side-channel map would split related state across
two values.

**7. "What if the rca insert succeeds but the work_items update
fails — do we have an orphan RCA?"**
No. Both writes are in the same pgx transaction. If either errors,
the engine's deferred `tx.Rollback()` fires and Postgres discards
everything. The `work_item.id` foreign key on the rca table would
let an orphan rca exist *conceptually* but never *temporally* —
they're written under one BEGIN.

**8. "Why use a `Reason` field on state_transitions?"**
Audit. When the post-mortem author looks at the timeline, "OPEN →
INVESTIGATING" is more useful as "OPEN → INVESTIGATING (reason:
'pager received')". v1 doesn't show this in the UI but the column
is there because adding it later would require a backfill.

**9. "How do you guarantee exactly one State Transition row per
actual transition?"**
The state_transition INSERT is in the SAME transaction as the
work_item UPDATE. If the UPDATE fails (or any subsequent step), the
INSERT rolls back. If the COMMIT succeeds, both writes are durable.
Postgres's atomicity guarantee handles this.

**10. "Why does ClosedState.OnEnter check `tctx.RCA != nil` again
when the engine already gated on CanTransitionTo?"**
Defense in depth. CanTransitionTo's job is "is this allowed?";
OnEnter's job is "do the side effects." A future bug where someone
calls OnEnter directly (skipping CanTransitionTo) would silently
zero out MTTR without the check. The cost is one nil comparison —
worth it.

**11. "Why are there no tests for the API handlers themselves
(httptest-style)?"**
The handlers are thin glue. The interesting logic (workflow.Engine,
State pattern, alert.Registry, RCA validation) is unit-tested. The
integration test in `pg/transition_test.go` exercises the engine
end-to-end. Phase 5 (or 6) may add full httptest coverage of the
handlers as part of the wider integration story.

**12. "What if Slack's webhook is down? Do we retry?"**
No. FR-6.4: alerts can't block workflow. The dispatch goroutine
times out at 5 seconds, logs the failure, and exits. The next
CREATED work item gets a fresh attempt. We accept losing some
notifications during outages because alerts are not source of
truth — the dashboard still shows the incident.

---

## 7. Things you should be able to do after Phase 4

- [ ] Walk through the State pattern interface from memory. What 3
      methods? What's special about ResolvedState?
- [ ] Add a fifth state ("ACKNOWLEDGED" between OPEN and
      INVESTIGATING) on a whiteboard. What files change? Should the
      transitions be OPEN→ACK→INVESTIGATING or OPEN→ACK + OPEN→INVESTIGATING?
- [ ] Run the PRD G3 scenario via curl (the script in the README).
      Hit each error path: invalid state name, missing RCA, short
      RCA, valid RCA → CLOSED + MTTR.
- [ ] Explain why we use SERIALIZABLE vs READ COMMITTED for state
      transitions. What's the failure mode at READ COMMITTED?
- [ ] Add a Microsoft Teams alerter. What's the minimum diff?
      (One new struct implementing `Alerter`, one new rule in main.go.)
- [ ] What goroutines does Phase 4 add? (Per-CREATED-work-item
      dispatch goroutines, ephemeral, bounded by AlertTimeout.)
- [ ] What's the difference between `errors.Is` and `errors.As`?
      When to use each in the workflow handler?
- [ ] Read `internal/workflow/state.go` and explain in plain English
      why `ResolvedState.CanTransitionTo` is the most important
      method in the file.

If those feel comfortable, Phase 4 is solid in your head.
