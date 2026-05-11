package timescale

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubeboiii/vellum/internal/model"
)

type MetricsWriter struct {
	pool *pgxpool.Pool
}

func NewMetricsWriter(pool *pgxpool.Pool) *MetricsWriter {
	return &MetricsWriter{pool: pool}
}

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

func (w *MetricsWriter) Count(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM signal_metrics`
	var n int
	if err := w.pool.QueryRow(ctx, q).Scan(&n); err != nil {
		return 0, fmt.Errorf("timescale: count signal_metrics: %w", err)
	}
	return n, nil
}
