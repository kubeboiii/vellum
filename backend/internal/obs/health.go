// Package obs holds the operator-facing surface: /health and the metrics
// ticker. Both depend on the pipeline (for queue depth + counters);
// Phase 3 added Pinger support so /health pings each persistence sink.
package obs

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/ims/internal/pipeline"
)

// Pinger is the contract a persistence package fulfils to participate
// in /health. Defined here (where it's consumed) per CLAUDE.md.
//
// Each implementation must honour the supplied context's deadline —
// /health enforces a per-dep timeout (default 500ms) so one stuck dep
// can't stall the whole response.
type Pinger interface {
	// Name is the lowercase JSON key under "dependencies"
	// (e.g. "postgres", "mongo", "redis"). Stable across phases.
	Name() string
	// Ping returns nil if the dependency is reachable.
	Ping(ctx context.Context) error
}

// Health holds the snapshot collectors plus the list of deps to ping.
type Health struct {
	started     atomic.Int64 // unix-nano of process start
	pipe        *pipeline.Pipeline
	deps        []Pinger
	pingTimeout time.Duration

	// criticalDeps are deps whose failure makes the whole service
	// "down" (returns 503). Other deps being down only marks the
	// service "degraded" (returns 200). For Phase 3: Postgres + Mongo
	// are critical; Redis is degraded (FR-3.6 fallback).
	criticalDeps map[string]struct{}
}

// HealthConfig lets the caller register the deps + per-ping timeout.
// Zero values fall back to: 500ms timeout, no deps, postgres+mongo
// considered critical.
type HealthConfig struct {
	Deps         []Pinger
	PingTimeout  time.Duration
	CriticalDeps []string
}

// NewHealth captures the process-start timestamp and wires the dep
// list. Pass an empty/zero config in Phase 2-style usage; Phase 3+
// fills it in.
func NewHealth(p *pipeline.Pipeline, cfg HealthConfig) *Health {
	if cfg.PingTimeout <= 0 {
		cfg.PingTimeout = 500 * time.Millisecond
	}
	if len(cfg.CriticalDeps) == 0 {
		cfg.CriticalDeps = []string{"postgres", "mongo"}
	}
	critical := make(map[string]struct{}, len(cfg.CriticalDeps))
	for _, n := range cfg.CriticalDeps {
		critical[n] = struct{}{}
	}
	h := &Health{
		pipe:         p,
		deps:         cfg.Deps,
		pingTimeout:  cfg.PingTimeout,
		criticalDeps: critical,
	}
	h.started.Store(time.Now().UnixNano())
	return h
}

// Handler returns the Gin handler for GET /health.
//
// Response codes:
//   - 200 healthy  : queue OK and all deps up.
//   - 200 degraded : queue OK but a non-critical dep (e.g. Redis) is down.
//   - 503 degraded : queue >95% full OR a critical dep (Postgres, Mongo) is down.
func (h *Health) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := h.pipe.Stats()
		uptime := time.Since(time.Unix(0, h.started.Load())).Seconds()

		deps, criticalDown, anyDown := h.pingAll(c.Request.Context())

		status := "healthy"
		code := http.StatusOK
		switch {
		case stats.Capacity > 0 && float64(stats.QueueDepth)/float64(stats.Capacity) > 0.95:
			status = "degraded"
			code = http.StatusServiceUnavailable
		case criticalDown:
			status = "degraded"
			code = http.StatusServiceUnavailable
		case anyDown:
			status = "degraded" // 200 — non-critical dep down
		}

		c.JSON(code, gin.H{
			"status":         status,
			"uptime_seconds": int(uptime),
			"queue_depth":    stats.QueueDepth,
			"queue_capacity": stats.Capacity,
			"counters": gin.H{
				"accepted":  stats.Accepted,
				"processed": stats.Processed,
				"dropped":   stats.Dropped,
				"errors":    stats.Errors,
			},
			"dependencies": deps,
		})
	}
}

// pingAll pings every registered dep, in parallel, with a per-dep
// timeout. Returns a JSON-shaped map plus two booleans telling the
// caller how to set the status. Pinging in parallel keeps the
// response time bounded by the slowest dep instead of the sum.
func (h *Health) pingAll(parentCtx context.Context) (map[string]gin.H, bool, bool) {
	type result struct {
		name      string
		status    string
		latencyMS int64
	}
	if len(h.deps) == 0 {
		return map[string]gin.H{}, false, false
	}

	ch := make(chan result, len(h.deps))
	for _, dep := range h.deps {
		dep := dep // shadow for goroutine capture
		go func() {
			ctx, cancel := context.WithTimeout(parentCtx, h.pingTimeout)
			defer cancel()
			start := time.Now()
			err := dep.Ping(ctx)
			r := result{
				name:      dep.Name(),
				latencyMS: time.Since(start).Milliseconds(),
				status:    "up",
			}
			if err != nil {
				r.status = "down"
			}
			ch <- r
		}()
	}

	deps := make(map[string]gin.H, len(h.deps))
	var criticalDown, anyDown bool
	for i := 0; i < len(h.deps); i++ {
		r := <-ch
		deps[r.name] = gin.H{"status": r.status, "latency_ms": r.latencyMS}
		if r.status == "down" {
			anyDown = true
			if _, ok := h.criticalDeps[r.name]; ok {
				criticalDown = true
			}
		}
	}
	return deps, criticalDown, anyDown
}
