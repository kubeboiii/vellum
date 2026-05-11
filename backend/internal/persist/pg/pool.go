// Package pg owns the Postgres connection pool and the WorkItem
// repository. Built on pgx v5's native pgxpool (NOT database/sql) per
// CLAUDE.md and 00-master-prd §12.
//
// One pool serves both the transactional tables (work_items,
// state_transitions) AND the Timescale hypertable (signal_metrics) —
// TimescaleDB is a Postgres extension, not a separate engine, so they
// share connections. See 01-architecture §3.2.
package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolConfig holds the knobs we expose. Pool sizing is the only one
// most callers will touch — defaults are reasonable for a single
// backend replica handling 10K signals/sec.
type PoolConfig struct {
	// DSN is the Postgres connection string, e.g.
	// postgres://ims:ims@localhost:5432/ims?sslmode=disable.
	DSN string
	// MaxConns caps the pool. Each worker can hold at most one
	// connection at a time, so we size for `worker_count * 1.5` plus
	// headroom for /health pings and admin queries. Default 32 is fine
	// for up to 20 workers (Phase 2 default on a 10-core box).
	MaxConns int32
	// ConnectTimeout bounds the initial Connect call. Beyond this we
	// fail-fast at startup rather than retrying silently.
	ConnectTimeout time.Duration
}

// NewPool builds a configured pgxpool. The caller is responsible for
// calling Close() at shutdown.
//
// We deliberately do NOT run migrations here — that's the migrate CLI's
// job. Mixing app code and schema management leaks DB-superuser
// permissions into the application's connection string, which is a
// classic prod-incident pattern.
func NewPool(ctx context.Context, cfg PoolConfig) (*pgxpool.Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("pg: DSN is required")
	}
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 32
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("pg: parse DSN: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	// pgx's default healthcheck (every 1 min) is fine — we don't need
	// to override it. The application's own /health does a Ping on a
	// 500ms timeout, which is the operator-facing check.

	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pg: connect: %w", err)
	}

	// Verify we can actually round-trip a query, not just open a TCP
	// connection — Postgres can accept connections before it's ready
	// to serve queries (especially right after container start).
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg: ping: %w", err)
	}
	return pool, nil
}
