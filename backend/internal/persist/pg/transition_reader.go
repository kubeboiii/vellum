package pg

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/vellum/internal/model"
)

type TransitionReader struct {
	pool *pgxpool.Pool
}

func NewTransitionReader(pool *pgxpool.Pool) *TransitionReader {
	return &TransitionReader{pool: pool}
}

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
