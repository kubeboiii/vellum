// Package obs holds the operator-facing surface: /health and the metrics
// ticker. Both depend on the pipeline (for queue depth + counters); /health
// will additionally ping each persistence sink once Phase 3 wires them.
//
// Phase 2 ships the structure but leaves the `dependencies` block empty —
// the goal is for Phase 3 to fill it in without touching the handler.
package obs

import (
	"net/http"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/ims/internal/pipeline"
)

// Phase 3 will add a Pinger interface here and a list of registered
// dependencies that /health iterates over. Deliberately omitted in
// Phase 2 — without a real persistence layer, the interface would be
// guesswork and we'd churn the shape on Phase 3.
//
// Health holds the snapshot collectors. We pass the pipeline directly
// rather than copying counters out — Stats() is cheap (4 atomic loads).
type Health struct {
	started atomic.Int64 // unix-nano of process start
	pipe    *pipeline.Pipeline
}

// NewHealth captures the process-start timestamp so /health can report
// uptime without keeping a side-channel timer. Pass a non-nil pipeline.
func NewHealth(p *pipeline.Pipeline) *Health {
	h := &Health{pipe: p}
	h.started.Store(time.Now().UnixNano())
	return h
}

// Handler returns the Gin handler for GET /health. Response shape mirrors
// the 01-architecture §11.1 sample. Phase 2 returns 503 only when the
// queue is >95% full, which is the early-warning signal the on-call cares
// about — Phase 3 will additionally 503 on any critical dep being down
// (FR-8.1).
func (h *Health) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		stats := h.pipe.Stats()
		uptime := time.Since(time.Unix(0, h.started.Load())).Seconds()

		status := "healthy"
		code := http.StatusOK
		// 95% threshold gives the operator time to react before the
		// pipeline starts 503-ing. Tuned by feel; could be configurable.
		if stats.Capacity > 0 && float64(stats.QueueDepth)/float64(stats.Capacity) > 0.95 {
			status = "degraded"
			code = http.StatusServiceUnavailable
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
			// Phase 3 fills this in. The key is present in Phase 2 so the
			// shape is stable for frontend/lib callers.
			"dependencies": gin.H{},
		})
	}
}
