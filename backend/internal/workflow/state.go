// Package workflow implements the Work Item lifecycle as a State
// pattern (00-master-prd FR-4.3, 01-architecture §7.2). The four
// states — Open, Investigating, Resolved, Closed — each live as their
// own type implementing the State interface. State-specific rules
// (what transitions are allowed, what side effects fire on entry)
// belong in the state type, NOT in a switch statement.
//
// CLAUDE.md design rule 3 ("RCA is required for CLOSED") is enforced
// in exactly one place: ResolvedState.CanTransitionTo(ClosedState).
// Any code that wants to close a Work Item routes through here.
//
// The transactional wrapper that actually persists transitions lives
// in engine.go — this file is the pure logic (zero I/O), which keeps
// the state machine easy to unit-test.
package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/kubeboiii/ims/internal/model"
)

// State is the contract every concrete state implements. We keep the
// surface tiny on purpose — three methods that name themselves.
//
//   - Name() is the persisted form (matches `work_items.status` text).
//   - CanTransitionTo(next, ctx) returns nil if the proposed transition
//     is allowed *given the current TransitionContext*. This is where
//     the RCA-required rule lives (in ResolvedState).
//   - OnEnter(ctx, wi) runs when a transition into this state commits.
//     ClosedState.OnEnter is where MTTR is computed and stamped on the
//     Work Item.
type State interface {
	Name() model.Status
	CanTransitionTo(next State, tctx TransitionContext) error
	OnEnter(ctx context.Context, wi *model.WorkItem, tctx TransitionContext) error
}

// TransitionContext is the bag of inputs a transition might need:
// the candidate RCA (only present for RESOLVED→CLOSED), the actor,
// an optional free-form reason. Keeping it as a struct (not function
// args) means we can add fields in later phases (e.g. impact_blast
// radius, user role) without changing every State signature.
type TransitionContext struct {
	RCA    *model.RCA
	Actor  string
	Reason string
}

// Sentinel errors. Handlers map these to HTTP codes:
//   - ErrInvalidTransition       → 409 Conflict
//   - ErrMissingRCA / ErrIncompleteRCA → 422 Unprocessable Entity (with field details)
var (
	ErrInvalidTransition = errors.New("workflow: transition not allowed from current state")
	ErrIncompleteRCA     = errors.New("workflow: RCA is incomplete; see fields")
)

// ErrMissingRCA is re-exported from the model package because the
// "no RCA at all" check is the very first thing in
// ResolvedState.CanTransitionTo. Surfacing it from this package too
// keeps callers (the API handler) from having to import `model` just
// to reference the sentinel.
var ErrMissingRCA = model.ErrMissingRCA

// ---- The four state types -------------------------------------------------

// OpenState: a work_item has just been created by the debouncer. Only
// legal transition is into INVESTIGATING; you can't skip ahead to
// RESOLVED or CLOSED (FR-4.2 — no backward, no skip).
type OpenState struct{}

func (OpenState) Name() model.Status { return model.StatusOpen }

func (OpenState) CanTransitionTo(next State, _ TransitionContext) error {
	if _, ok := next.(InvestigatingState); ok {
		return nil
	}
	return fmt.Errorf("%w: OPEN -> %s", ErrInvalidTransition, next.Name())
}

// OnEnter for OpenState is a no-op — the work_item row is created in
// the processor's CREATED branch with status=OPEN, so by the time
// anyone calls OnEnter on OpenState we've already done it. We never
// transition INTO OpenState from another state (FR-4.2).
func (OpenState) OnEnter(_ context.Context, _ *model.WorkItem, _ TransitionContext) error {
	return nil
}

// InvestigatingState: someone is actively diagnosing. Can only move
// forward to RESOLVED.
type InvestigatingState struct{}

func (InvestigatingState) Name() model.Status { return model.StatusInvestigating }

func (InvestigatingState) CanTransitionTo(next State, _ TransitionContext) error {
	if _, ok := next.(ResolvedState); ok {
		return nil
	}
	return fmt.Errorf("%w: INVESTIGATING -> %s", ErrInvalidTransition, next.Name())
}

func (InvestigatingState) OnEnter(_ context.Context, _ *model.WorkItem, _ TransitionContext) error {
	return nil
}

// ResolvedState: the fix is in place; awaiting RCA before close. This
// is the ONE state whose CanTransitionTo does real work (RCA gating).
// Every other state's check is structural ("is this the next state?").
type ResolvedState struct{}

func (ResolvedState) Name() model.Status { return model.StatusResolved }

// CanTransitionTo for ResolvedState is THE place CLAUDE.md design
// rule 3 lives:
//
//	"RCA is required for CLOSED. ResolvedState.CanTransitionTo
//	 rejects with ErrMissingRCA or ErrIncompleteRCA if the RCA is
//	 missing or invalid. This rule lives in exactly one place."
//
// A reviewer looking for "where do we enforce the RCA rule?" finds
// it here, no scavenger hunt required.
func (ResolvedState) CanTransitionTo(next State, tctx TransitionContext) error {
	if _, ok := next.(ClosedState); !ok {
		return fmt.Errorf("%w: RESOLVED -> %s", ErrInvalidTransition, next.Name())
	}
	if tctx.RCA == nil {
		return ErrMissingRCA
	}
	if errs := tctx.RCA.Validate(); len(errs) > 0 {
		// Wrap the validation errors so the handler can pull them via
		// errors.As if it wants the field-level detail.
		return &IncompleteRCAError{Fields: errs}
	}
	return nil
}

func (ResolvedState) OnEnter(_ context.Context, _ *model.WorkItem, _ TransitionContext) error {
	return nil
}

// ClosedState: terminal. No outgoing transitions. OnEnter is where
// MTTR is computed and stamped on the work_item.
type ClosedState struct{}

func (ClosedState) Name() model.Status { return model.StatusClosed }

func (ClosedState) CanTransitionTo(next State, _ TransitionContext) error {
	return fmt.Errorf("%w: CLOSED is terminal -> %s", ErrInvalidTransition, next.Name())
}

// OnEnter for ClosedState computes MTTR and stamps the closure
// timestamp. This is the OTHER place state-specific behaviour lives;
// 01-architecture §7.2 calls this out explicitly:
//
//	"Properties of this design: MTTR computation is automatic —
//	 happens when entering ClosedState."
//
// The engine calls this from inside the SERIALIZABLE transaction
// (engine.go), so if OnEnter panics or returns an error, the whole
// transition rolls back and the work_item stays in RESOLVED.
func (ClosedState) OnEnter(_ context.Context, wi *model.WorkItem, tctx TransitionContext) error {
	if tctx.RCA == nil {
		// Defense in depth — ResolvedState.CanTransitionTo already
		// rejected this above. Reaching here means the engine called
		// OnEnter without honouring the canTransition gate. That's a
		// programming bug, not a user error, so we panic in tests
		// (return error in production).
		return fmt.Errorf("workflow: ClosedState.OnEnter called without RCA (engine bug)")
	}
	mttr := tctx.RCA.MTTRSeconds()
	wi.MTTRSeconds = &mttr
	closedAt := tctx.RCA.IncidentEnd
	wi.ClosedAt = &closedAt
	wi.IncidentStart = &tctx.RCA.IncidentStart
	wi.IncidentEnd = &tctx.RCA.IncidentEnd
	return nil
}

// ---- IncompleteRCAError carries field-level details -------------------------

// IncompleteRCAError wraps the per-field validation errors returned
// by model.RCA.Validate so handlers can produce a 422 response with
// `errors: [{field, error}, ...]` shape.
type IncompleteRCAError struct {
	Fields []model.FieldError
}

func (e *IncompleteRCAError) Error() string {
	return ErrIncompleteRCA.Error()
}

// Unwrap so errors.Is(err, ErrIncompleteRCA) works.
func (e *IncompleteRCAError) Unwrap() error { return ErrIncompleteRCA }

// ---- Helper: build a State from a stored status string ---------------------

// FromStatus parses the status text persisted in `work_items.status`
// back into the concrete State type. Returns an error for unknown
// values so a corrupted DB row doesn't silently re-route through an
// unexpected branch.
func FromStatus(s model.Status) (State, error) {
	switch s {
	case model.StatusOpen:
		return OpenState{}, nil
	case model.StatusInvestigating:
		return InvestigatingState{}, nil
	case model.StatusResolved:
		return ResolvedState{}, nil
	case model.StatusClosed:
		return ClosedState{}, nil
	}
	return nil, fmt.Errorf("workflow: unknown status %q", s)
}
