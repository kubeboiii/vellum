package pg

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kubeboiii/vellum/internal/model"
)

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

func (r *WorkItemRepository) ListActive(ctx context.Context, limit int) ([]model.WorkItem, error) {
	if limit <= 0 {
		limit = 100
	}

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

func (r *WorkItemRepository) ListClosed(ctx context.Context, limit int) ([]model.WorkItem, error) {
	if limit <= 0 {
		limit = 100
	}
	const q = `
		SELECT id, component_id, component_type, severity, status,
		       signal_count, first_signal_ts, last_signal_ts,
		       mttr_seconds, incident_start, incident_end, closed_at,
		       created_at, updated_at
		  FROM work_items
		 WHERE status = 'CLOSED'
		 ORDER BY closed_at DESC NULLS LAST
		 LIMIT $1
	`
	rows, err := r.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("pg: list closed: %w", err)
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
		return nil, fmt.Errorf("pg: list closed rows: %w", err)
	}
	return out, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

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
