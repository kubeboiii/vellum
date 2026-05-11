package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/ims/internal/model"
)

// TransitionReader is the read-side gateway for the state_transitions
// audit table. The write path lives inside the workflow.Engine's
// transaction (workItemTx.InsertStateTransition); reads happen outside
// any transaction for the Phase 5 detail-page timeline.
type TransitionReader struct {
	pool *pgxpool.Pool
}

// NewTransitionReader wires the reader to the pgx pool.
func NewTransitionReader(pool *pgxpool.Pool) *TransitionReader {
	return &TransitionReader{pool: pool}
}

// ListByWorkItem returns the audit rows for one Work Item, chronological
// ascending. Powers the detail page's "Timeline" panel (THEME.md §7.2,
// PRD FR-7.2).
//
// Volume is tiny — a work_item has at most ~3 transitions in v1 (OPEN
// → INVESTIGATING → RESOLVED → CLOSED), so we don't paginate.
func (r *TransitionReader) ListByWorkItem(ctx context.Context, workItemID uuid.UUID) ([]model.StateTransition, error) {
	const q = `
		SELECT id, work_item_id, from_state, to_state,
		       COALESCE(reason, ''), COALESCE(actor, ''), created_at
		  FROM state_transitions
		 WHERE work_item_id = $1
		 ORDER BY created_at ASC
	`
	rows, err := r.pool.Query(ctx, q, workItemID)
	if err != nil {
		return nil, fmt.Errorf("pg: list transitions: %w", err)
	}
	defer rows.Close()

	out := make([]model.StateTransition, 0, 4)
	for rows.Next() {
		var t model.StateTransition
		var fromState, toState string
		if err := rows.Scan(
			&t.ID, &t.WorkItemID,
			&fromState, &toState,
			&t.Reason, &t.Actor,
			&t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("pg: scan transition: %w", err)
		}
		t.FromState = model.Status(fromState)
		t.ToState = model.Status(toState)
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg: rows err: %w", err)
	}
	return out, nil
}
