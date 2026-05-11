// Package timescale writes to the `signal_metrics` hypertable.
// TimescaleDB is a Postgres extension, so this package reuses the
// existing pgxpool — there's no second connection pool or driver
// (01-architecture §3.2: "one less container and one less driver").
//
// Hypertables look like ordinary tables to your code; the underlying
// auto-partitioning by `ts` happens transparently. We just INSERT.
package timescale

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/ims/internal/model"
)

// MetricsWriter inserts one row per signal into signal_metrics. Used
// for (Phase 6+) per-minute rollups, MTTR distribution, etc.
type MetricsWriter struct {
	pool *pgxpool.Pool
}

// NewMetricsWriter wires the writer to a pgx pool. The pool is the
// SAME pool used by the pg package — Timescale lives inside it.
func NewMetricsWriter(pool *pgxpool.Pool) *MetricsWriter {
	return &MetricsWriter{pool: pool}
}

// Insert writes one metric row. Tied to a single signal, but the table
// is keyed only on (ts, component_type, severity, work_item_id) so a
// hypothetical batched insert (Phase 6 optimization?) would work fine.
func (w *MetricsWriter) Insert(ctx context.Context, sig model.Signal, workItemID uuid.UUID) error {
	const q = `
		INSERT INTO signal_metrics (ts, component_type, severity, work_item_id, count)
		VALUES ($1, $2, $3, $4, 1)
	`
	_, err := w.pool.Exec(ctx, q,
		sig.Timestamp,
		string(sig.ComponentType),
		string(sig.Severity),
		workItemID,
	)
	if err != nil {
		return fmt.Errorf("timescale: insert signal_metric: %w", err)
	}
	return nil
}

// Count returns the number of metric rows recorded. Test/acceptance
// helper only.
func (w *MetricsWriter) Count(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM signal_metrics`
	var n int
	if err := w.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("timescale: count signal_metrics: %w", err)
	}
	return n, nil
}
