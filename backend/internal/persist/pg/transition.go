package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kubeboiii/ims/internal/model"
)

// BeginTx opens a SERIALIZABLE Postgres transaction and returns a
// `workItemTx` handle the workflow.Engine drives. SERIALIZABLE is
// CLAUDE.md design rule 2 and 01-architecture §12: it protects the
// state_transitions audit table from phantom reads (two concurrent
// closes for the same WI both seeing "no prior CLOSED transition" and
// both writing one). The contention is low (transitions are
// human-driven) so the perf cost is negligible.
func (r *WorkItemRepository) BeginTx(ctx context.Context) (*workItemTx, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("pg: begin tx: %w", err)
	}
	return &workItemTx{tx: tx}, nil
}

// workItemTx is the concrete `workflow.Tx`. It holds an open pgx
// transaction and provides the locking + write operations the engine
// needs. Each handler call gets a fresh instance — they aren't reused
// across requests.
type workItemTx struct {
	tx pgx.Tx
}

// LockWorkItem runs SELECT ... FOR UPDATE on the target row. This is
// the second half of design rule 2 (01-architecture §7.2.1): the row
// is locked for the rest of the transaction, so a concurrent
// transaction attempting the same SELECT FOR UPDATE blocks until we
// commit/rollback. Result: two simultaneous PATCH .../state calls on
// the same WI serialize at the row level; whichever commits first
// wins, the other re-evaluates its state and gets the right answer.
func (t *workItemTx) LockWorkItem(ctx context.Context, id uuid.UUID) (model.WorkItem, error) {
	const q = `
		SELECT id, component_id, component_type, severity, status,
		       signal_count, first_signal_ts, last_signal_ts,
		       mttr_seconds, incident_start, incident_end, closed_at,
		       created_at, updated_at
		  FROM work_items
		 WHERE id = $1
		 FOR UPDATE
	`
	var wi model.WorkItem
	var componentType, severity, status string
	err := t.tx.QueryRow(ctx, q, id).Scan(
		&wi.ID,
		&wi.ComponentID,
		&componentType,
		&severity,
		&status,
		&wi.SignalCount,
		&wi.FirstSignalTS,
		&wi.LastSignalTS,
		&wi.MTTRSeconds,
		&wi.IncidentStart,
		&wi.IncidentEnd,
		&wi.ClosedAt,
		&wi.CreatedAt,
		&wi.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.WorkItem{}, ErrNotFound
		}
		return model.WorkItem{}, fmt.Errorf("pg: lock work_item: %w", err)
	}
	wi.ComponentType = model.ComponentType(componentType)
	wi.Severity = model.Severity(severity)
	wi.Status = model.Status(status)
	return wi, nil
}

// UpdateWorkItemStateAndMTTR persists the post-transition fields. We
// always set status + updated_at; the four nullable timestamps go
// through as-is (NULL stays NULL, set stays set).
//
// We accept the full WorkItem rather than just (id, status, ...) so
// ClosedState.OnEnter can populate MTTR + ClosedAt + IncidentStart +
// IncidentEnd in memory and have them all persisted in one UPDATE.
func (t *workItemTx) UpdateWorkItemStateAndMTTR(ctx context.Context, wi model.WorkItem) error {
	const q = `
		UPDATE work_items
		   SET status         = $2,
		       mttr_seconds   = $3,
		       incident_start = $4,
		       incident_end   = $5,
		       closed_at      = $6,
		       updated_at     = now()
		 WHERE id = $1
	`
	tag, err := t.tx.Exec(ctx, q,
		wi.ID,
		string(wi.Status),
		wi.MTTRSeconds,
		wi.IncidentStart,
		wi.IncidentEnd,
		wi.ClosedAt,
	)
	if err != nil {
		return fmt.Errorf("pg: update work_item state: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// InsertStateTransition writes one row to the audit table inside the
// same transaction. FR-4.4: "The Work Item update and the transition
// row write are in a single database transaction."
func (t *workItemTx) InsertStateTransition(ctx context.Context, st model.StateTransition) error {
	const q = `
		INSERT INTO state_transitions
		    (id, work_item_id, from_state, to_state, reason, actor)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''))
	`
	_, err := t.tx.Exec(ctx, q,
		st.ID, st.WorkItemID,
		string(st.FromState), string(st.ToState),
		st.Reason, st.Actor,
	)
	if err != nil {
		return fmt.Errorf("pg: insert state_transition: %w", err)
	}
	return nil
}

// InsertRCA writes the RCA row inside the same transaction as the
// CLOSE transition. UNIQUE constraint on work_item_id means a duplicate
// insert will fail — that becomes the "already closed" race protection
// at the DB level.
func (t *workItemTx) InsertRCA(ctx context.Context, rca model.RCA) error {
	const q = `
		INSERT INTO rca
		    (id, work_item_id, incident_start, incident_end,
		     root_cause_category, fix_applied, prevention_steps,
		     submitted_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := t.tx.Exec(ctx, q,
		rca.ID, rca.WorkItemID,
		rca.IncidentStart, rca.IncidentEnd,
		string(rca.RootCauseCategory),
		rca.FixApplied, rca.PreventionSteps,
		rca.SubmittedBy, rca.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("pg: insert rca: %w", err)
	}
	return nil
}

// Commit ends the transaction successfully. After this the rollback
// `defer` in the engine is a no-op (pgx handles that).
func (t *workItemTx) Commit() error {
	if err := t.tx.Commit(context.Background()); err != nil {
		return fmt.Errorf("pg: commit: %w", err)
	}
	return nil
}

// Rollback ends the transaction without persisting. Idempotent.
func (t *workItemTx) Rollback() error {
	_ = t.tx.Rollback(context.Background())
	return nil
}
