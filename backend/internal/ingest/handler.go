// Package ingest holds the HTTP entry points and the per-source rate
// limiter. The package depends on `pipeline` (to enqueue) and `model` (the
// wire schema) but nothing below; this enforces the inward-dependency rule
// from 01-architecture §10.1.
//
// Phase 2 covers HTTP only. The gRPC server (Phase 5) will live here too
// and share the same pipeline.
package ingest

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/ims/internal/model"
	"github.com/kubeboiii/ims/internal/pipeline"
)

// Submitter is the narrow contract the handler needs from the pipeline.
// Defined here, where it's consumed (per CLAUDE.md "small interfaces,
// defined where consumed"). Lets us swap in a fake in tests.
type Submitter interface {
	Submit(model.Signal) bool
}

// retryAfterMillis is the hint we put in the 503 body when the queue is
// saturated. 100ms = roughly 1s of nominal load at 10K/sec ÷ a 50K queue,
// i.e. by the time the caller retries, workers will have drained ~10K
// slots. The HTTP `Retry-After` header is in seconds; we round up to 1.
const retryAfterMillis = 100

// NewHandler returns the Gin handler for POST /v1/signals. It captures
// `now` so tests can inject a frozen clock. Production callers pass
// `time.Now`.
func NewHandler(p Submitter, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sig model.Signal
		if err := c.ShouldBindJSON(&sig); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
			return
		}
		if err := sig.Validate(); err != nil {
			// Validation errors are user errors (400) not server errors.
			// errors.Is lets us not leak the wrapped %q value into the
			// response, but we do want the message — it tells the caller
			// which field is wrong.
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sig.ApplyDefaults(now())

		// THE HOT PATH. Single non-blocking enqueue. Do NOT add work here.
		if !p.Submit(sig) {
			c.Header("Retry-After", "1")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":          "ingestion queue full",
				"retry_after_ms": retryAfterMillis,
			})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"status":    "accepted",
			"signal_id": sig.SignalID,
		})
	}
}

// Sentinel returned by RegisterRoutes if the caller hands us a nil
// pipeline — better a clear constructor-time error than a nil-pointer
// panic on the first request.
var ErrNilPipeline = errors.New("ingest: pipeline must not be nil")

// RegisterRoutes mounts the v1 ingestion endpoints onto the supplied
// router group. Keeping route wiring in this package (rather than in
// cmd/ims/main.go) keeps main.go thin and ensures the package owns its
// URL prefix.
func RegisterRoutes(rg *gin.RouterGroup, p *pipeline.Pipeline, limiter gin.HandlerFunc) error {
	if p == nil {
		return ErrNilPipeline
	}
	handler := NewHandler(p, time.Now)
	if limiter != nil {
		rg.POST("/signals", limiter, handler)
	} else {
		rg.POST("/signals", handler)
	}
	return nil
}
