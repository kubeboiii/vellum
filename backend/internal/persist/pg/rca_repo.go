package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/ims/internal/model"
)

// RCARepository is the read-side gateway for the `rca` table. Writes
// happen inside the workflow engine's transaction (workItemTx.InsertRCA)
// because they MUST be atomic with the RESOLVED → CLOSED transition.
// Reads — surfacing the RCA on the incident detail page — happen
// outside any transaction, which is what this struct handles.
type RCARepository struct {
	pool *pgxpool.Pool
}

// NewRCARepository wires the repo to a live pool.
func NewRCARepository(pool *pgxpool.Pool) *RCARepository {
	return &RCARepository{pool: pool}
}

// GetByWorkItemID returns the RCA attached to the given Work Item, or
// ErrNotFound if no RCA exists yet (i.e., the WI hasn't been closed).
// Phase 5's detail page calls this when it sees `status = CLOSED`.
func (r *RCARepository) GetByWorkItemID(ctx context.Context, workItemID uuid.UUID) (model.RCA, error) {
	const q = `
		SELECT id, work_item_id, incident_start, incident_end,
		       root_cause_category, fix_applied, prevention_steps,
		       submitted_by, created_at
		  FROM rca
		 WHERE work_item_id = $1
	`
	var rca model.RCA
	var category string
	err := r.pool.QueryRow(ctx, q, workItemID).Scan(
		&rca.ID, &rca.WorkItemID,
		&rca.IncidentStart, &rca.IncidentEnd,
		&category,
		&rca.FixApplied, &rca.PreventionSteps,
		&rca.SubmittedBy, &rca.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.RCA{}, ErrNotFound
		}
		return model.RCA{}, fmt.Errorf("pg: get rca: %w", err)
	}
	rca.RootCauseCategory = model.RootCauseCategory(category)
	return rca, nil
}
