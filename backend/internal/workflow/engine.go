package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/ims/internal/model"
)

// TxRunner is the narrow contract the engine needs from the
// persistence layer to wrap a single transition in one Postgres
// transaction. Defined here (where it's consumed) per CLAUDE.md;
// `pg.WorkItemRepository` implements it (see Phase 4's extensions).
//
// The semantics matter:
//
//   - BeginTx MUST open a SERIALIZABLE transaction (CLAUDE.md design
//     rule 2 + 00-master-prd §4.4.4).
//   - Within the transaction, LockWorkItem MUST issue a SELECT FOR
//     UPDATE on the work_items row — this is what prevents two
//     concurrent transitions on the same work_item from both
//     succeeding (01-architecture §7.2.1).
//   - InsertStateTransition writes the audit row. Required by FR-4.4.
//   - UpdateWorkItemStateAndMTTR persists the new status (and, on
//     close, the MTTR + ClosedAt fields populated by
//     ClosedState.OnEnter).
//   - InsertRCA is only called on the RESOLVED → CLOSED path; the
//     engine's Close method takes both jobs (RCA + transition) and
//     runs them in the same Tx so they're atomic together.
//   - Commit or Rollback ends the transaction.
type TxRunner interface {
	BeginTx(ctx context.Context) (Tx, error)
}

// Tx is the abstract handle the engine drives during a transition.
// `pg.workItemTx` is the concrete implementation. Tests fake this.
type Tx interface {
	LockWorkItem(ctx context.Context, id uuid.UUID) (model.WorkItem, error)
	UpdateWorkItemStateAndMTTR(ctx context.Context, wi model.WorkItem) error
	InsertStateTransition(ctx context.Context, t model.StateTransition) error
	InsertRCA(ctx context.Context, rca model.RCA) error
	Commit() error
	Rollback() error
}

// ErrWorkItemNotFound mirrors the pg-side sentinel so the API handler
// can map it to 404 without importing pg directly.
var ErrWorkItemNotFound = errors.New("workflow: work item not found")

// Engine is the orchestrator. Constructed once at startup with a
// TxRunner; one instance handles every transition in the process.
type Engine struct {
	tx TxRunner
}

// NewEngine builds an engine wired to the given TxRunner.
func NewEngine(tx TxRunner) *Engine {
	return &Engine{tx: tx}
}

// Transition advances `id` to `next` state in one SERIALIZABLE
// transaction. The flow (01-architecture §7.2.1):
//
//  1. BEGIN SERIALIZABLE
//  2. SELECT * FROM work_items WHERE id = $1 FOR UPDATE
//  3. Construct current State from row.status; call CanTransitionTo
//  4. If allowed: call next.OnEnter (may set MTTR/ClosedAt);
//     UPDATE work_items; INSERT into state_transitions
//  5. COMMIT
//
// On any error after BeginTx, we Rollback before returning. The
// caller never sees a half-committed state.
func (e *Engine) Transition(ctx context.Context, id uuid.UUID, next State, tctx TransitionContext) (model.WorkItem, error) {
	tx, err := e.tx.BeginTx(ctx)
	if err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: begin tx: %w", err)
	}
	// Rollback is a no-op once Commit succeeded (pgx behaviour), so
	// defer-rollback is safe even on the happy path.
	defer func() { _ = tx.Rollback() }()

	wi, err := tx.LockWorkItem(ctx, id)
	if err != nil {
		return model.WorkItem{}, err
	}

	current, err := FromStatus(wi.Status)
	if err != nil {
		return model.WorkItem{}, err
	}
	if err := current.CanTransitionTo(next, tctx); err != nil {
		return model.WorkItem{}, err
	}

	if err := next.OnEnter(ctx, &wi, tctx); err != nil {
		return model.WorkItem{}, err
	}
	wi.Status = next.Name()

	if err := tx.UpdateWorkItemStateAndMTTR(ctx, wi); err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: update work_item: %w", err)
	}
	transition := model.StateTransition{
		ID:         uuid.New(),
		WorkItemID: wi.ID,
		FromState:  current.Name(),
		ToState:    next.Name(),
		Reason:     tctx.Reason,
		Actor:      tctx.Actor,
	}
	if err := tx.InsertStateTransition(ctx, transition); err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: insert transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: commit: %w", err)
	}
	return wi, nil
}

// CloseWithRCA is the compound RESOLVED → CLOSED path: it inserts the
// RCA row AND runs the transition AND computes MTTR — all in one
// transaction. If anything fails, nothing persists.
//
// The State pattern still gates the close (ResolvedState.CanTransitionTo
// runs as usual), so the "no RCA → no close" rule is enforced by the
// SAME code path as a manual `PATCH /state` would be. There's no
// duplication of business logic across the two endpoints.
func (e *Engine) CloseWithRCA(ctx context.Context, workItemID uuid.UUID, rca *model.RCA, actor string) (model.WorkItem, model.RCA, error) {
	if rca == nil {
		return model.WorkItem{}, model.RCA{}, ErrMissingRCA
	}

	tx, err := e.tx.BeginTx(ctx)
	if err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	wi, err := tx.LockWorkItem(ctx, workItemID)
	if err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}

	// Default the RCA's incident_start to the WI's first signal
	// timestamp, and incident_end to "now" (the close time), per
	// 00-master-prd §4.5. MTTR is computed from these two in
	// ClosedState.OnEnter, so they MUST be a real duration apart.
	rca.WorkItemID = wi.ID
	rca.ApplyDefaults(wi.FirstSignalTS, time.Now().UTC())

	current, err := FromStatus(wi.Status)
	if err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}
	tctx := TransitionContext{RCA: rca, Actor: actor}
	if err := current.CanTransitionTo(ClosedState{}, tctx); err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}

	// OnEnter for ClosedState stamps MTTR + ClosedAt + incident
	// timestamps onto the WI struct (still in memory).
	if err := (ClosedState{}).OnEnter(ctx, &wi, tctx); err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}
	wi.Status = model.StatusClosed

	if err := tx.InsertRCA(ctx, *rca); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: insert rca: %w", err)
	}
	if err := tx.UpdateWorkItemStateAndMTTR(ctx, wi); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: update work_item: %w", err)
	}
	transition := model.StateTransition{
		ID:         uuid.New(),
		WorkItemID: wi.ID,
		FromState:  current.Name(),
		ToState:    model.StatusClosed,
		Reason:     "RCA submitted",
		Actor:      actor,
	}
	if err := tx.InsertStateTransition(ctx, transition); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: insert transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: commit: %w", err)
	}
	return wi, *rca, nil
}
