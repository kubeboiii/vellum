package obs

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/vellum/internal/pipeline"
)

type Pinger interface {

	Name() string

	Ping(ctx context.Context) error
}

type Health struct {
	started     atomic.Int64
	pipe        *pipeline.Pipeline
	deps        []Pinger
	pingTimeout time.Duration

	criticalDeps map[string]struct{}
}

type HealthConfig struct {
	Deps         []Pinger
	PingTimeout  time.Duration
	CriticalDeps []string
}

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
			status = "degraded"
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
		dep := dep
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
