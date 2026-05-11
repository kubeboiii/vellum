package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/vellum/internal/model"
)

type WorkItemRepository struct {
	pool *pgxpool.Pool
}

func NewWorkItemRepository(pool *pgxpool.Pool) *WorkItemRepository {
	return &WorkItemRepository{pool: pool}
}

var ErrNotFound = errors.New("pg: work item not found")

func (r *WorkItemRepository) Insert(ctx context.Context, wi model.WorkItem) error {
	const q = `
		INSERT INTO work_items (
			id, component_id, component_type, severity, status,
			signal_count, first_signal_ts, last_signal_ts,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.pool.Exec(ctx, q,
		wi.ID, wi.ComponentID, string(wi.ComponentType), string(wi.Severity), string(wi.Status),
		wi.SignalCount, wi.FirstSignalTS, wi.LastSignalTS,
		wi.CreatedAt, wi.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("pg: insert work_item: %w", err)
	}
	return nil
}

func (r *WorkItemRepository) IncrementSignalCount(ctx context.Context, id uuid.UUID, signalTS time.Time) error {
	const q = `
		UPDATE work_items
		   SET signal_count   = signal_count + 1,
		       last_signal_ts = GREATEST(last_signal_ts, $2),
		       updated_at     = now()
		 WHERE id = $1
	`

	tag, err := r.pool.Exec(ctx, q, id, signalTS)
	if err != nil {
		return fmt.Errorf("pg: increment signal_count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WorkItemRepository) CountByComponent(ctx context.Context, componentID string) (int, error) {
	const q = `SELECT COUNT(*) FROM work_items WHERE component_id = $1`
	var n int
	err := r.pool.QueryRow(ctx, q, componentID).Scan(&n)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("pg: count work_items: %w", err)
	}
	return n, nil
}

func (r *WorkItemRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *WorkItemRepository) Name() string { return "postgres" }
