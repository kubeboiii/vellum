package pg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/ims/internal/model"
)

// WorkItemRepository is the gateway for everything that mutates or
// reads the `work_items` table. Defined as a struct (not an interface)
// here because Go's idiom is to define interfaces where they're
// CONSUMED — the processor package and the workflow package (Phase 4)
// will each define narrow interfaces that this struct happens to
// satisfy.
type WorkItemRepository struct {
	pool *pgxpool.Pool
}

// NewWorkItemRepository wires the repo to a live pool.
func NewWorkItemRepository(pool *pgxpool.Pool) *WorkItemRepository {
	return &WorkItemRepository{pool: pool}
}

// ErrNotFound is returned when an Update affects 0 rows. Surfaces as a
// "lost the debounce race" condition: the row we expected to UPDATE
// doesn't exist (could be because the window expired and the cache
// pointed at a now-deleted work_item — unlikely in practice but worth
// handling cleanly).
var ErrNotFound = errors.New("pg: work item not found")

// Insert writes a new work_items row in OPEN state. The ID is provided
// by the caller (it's the candidate UUID the processor sent to the
// Redis Lua script). Returns an error if the insert fails for any
// reason; uniqueness is enforced by the PRIMARY KEY constraint, so
// re-inserting the same ID would conflict — that's a logic bug on the
// caller side, not something we silently swallow.
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

// IncrementSignalCount bumps signal_count by 1 and refreshes
// last_signal_ts. Called when the Lua debounce script returns
// action=JOINED.
//
// We do this with an UPDATE rather than re-fetching + re-saving because
// (a) it's one network round-trip instead of two, and (b) it's
// concurrency-safe at the row level — Postgres serializes the writes
// to the same row via per-row locks. Concurrent JOINED updates from
// multiple workers can't lose a count.
func (r *WorkItemRepository) IncrementSignalCount(ctx context.Context, id uuid.UUID, signalTS time.Time) error {
	const q = `
		UPDATE work_items
		   SET signal_count   = signal_count + 1,
		       last_signal_ts = GREATEST(last_signal_ts, $2),
		       updated_at     = now()
		 WHERE id = $1
	`
	// GREATEST guards against out-of-order signals arriving at workers
	// (they're parallel — signal B accepted before A may be processed
	// after A). last_signal_ts should monotonically reflect the
	// wall-clock latest signal, not the latest-to-arrive.
	tag, err := r.pool.Exec(ctx, q, id, signalTS)
	if err != nil {
		return fmt.Errorf("pg: increment signal_count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByComponent is a test/acceptance helper: how many open work_items
// exist for this component_id. Used by the Phase 3 acceptance demo to
// prove "200 signals → 1-3 work_items". Not on the hot path.
func (r *WorkItemRepository) CountByComponent(ctx context.Context, componentID string) (int, error) {
	const q = `SELECT COUNT(*) FROM work_items WHERE component_id = $1`
	var n int
	err := r.pool.QueryRow(ctx, q, componentID).Scan(&n)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("pg: count work_items: %w", err)
	}
	return n, nil
}

// Ping is the /health probe. Honors the supplied context timeout.
func (r *WorkItemRepository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// Name identifies this dep in /health responses.
func (r *WorkItemRepository) Name() string { return "postgres" }
