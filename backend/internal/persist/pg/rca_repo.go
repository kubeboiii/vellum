package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/vellum/internal/model"
)

type RCARepository struct {
	pool *pgxpool.Pool
}

func NewRCARepository(pool *pgxpool.Pool) *RCARepository {
	return &RCARepository{pool: pool}
}

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
