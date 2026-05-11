package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kubeboiii/ims/internal/model"
)

// GetByID reads a single work_item by primary key. Non-locking,
// non-transactional — used by GET /v1/incidents/:id and by tests.
// Returns ErrNotFound if no row matches.
func (r *WorkItemRepository) GetByID(ctx context.Context, id uuid.UUID) (model.WorkItem, error) {
	const q = `
		SELECT id, component_id, component_type, severity, status,
		       signal_count, first_signal_ts, last_signal_ts,
		       mttr_seconds, incident_start, incident_end, closed_at,
		       created_at, updated_at
		  FROM work_items
		 WHERE id = $1
	`
	return scanWorkItem(r.pool.QueryRow(ctx, q, id))
}

// ListActive returns the non-CLOSED Work Items sorted by severity
// (P0 first) then by last_signal_ts DESC. Powers GET /v1/incidents
// (the live feed). The partial index `idx_work_items_active` makes
// this scan small even with months of CLOSED rows in the table.
//
// `limit` caps the result; the live feed will pass something like
// 100 to keep the response small.
func (r *WorkItemRepository) ListActive(ctx context.Context, limit int) ([]model.WorkItem, error) {
	if limit <= 0 {
		limit = 100
	}
	// Ordering by severity: P0 < P1 < P2 < P3 lexicographically, so
	// ASC sorts P0 first. Lucky alignment of the enum to lex order.
	const q = `
		SELECT id, component_id, component_type, severity, status,
		       signal_count, first_signal_ts, last_signal_ts,
		       mttr_seconds, incident_start, incident_end, closed_at,
		       created_at, updated_at
		  FROM work_items
		 WHERE status <> 'CLOSED'
		 ORDER BY severity ASC, last_signal_ts DESC
		 LIMIT $1
	`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("pg: list active: %w", err)
	}
	defer rows.Close()

	out := make([]model.WorkItem, 0, limit)
	for rows.Next() {
		wi, err := scanWorkItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, wi)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg: list active rows: %w", err)
	}
	return out, nil
}

// rowScanner is the narrow interface pgx.Row and pgx.Rows both satisfy
// for our purposes — lets scanWorkItem be shared between GetByID
// (single row) and ListActive (iteration).
type rowScanner interface {
	Scan(dest ...any) error
}

// scanWorkItem decodes one row into a model.WorkItem. Centralises the
// column order so a schema change is a one-file edit (vs. four).
func scanWorkItem(s rowScanner) (model.WorkItem, error) {
	var wi model.WorkItem
	var componentType, severity, status string
	err := s.Scan(
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
		return model.WorkItem{}, fmt.Errorf("pg: scan work_item: %w", err)
	}
	wi.ComponentType = model.ComponentType(componentType)
	wi.Severity = model.Severity(severity)
	wi.Status = model.Status(status)
	return wi, nil
}
