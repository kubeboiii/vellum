package ingest

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/vellum/internal/model"
	"github.com/kubeboiii/vellum/internal/pipeline"
)

type Submitter interface {
	Submit(model.Signal) bool
}

const retryAfterMillis = 100

func NewHandler(p Submitter, now func() time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		var sig model.Signal
		if err := c.ShouldBindJSON(&sig); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
			return
		}
		if err := sig.Validate(); err != nil {

			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		sig.ApplyDefaults(now())

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

var ErrNilPipeline = errors.New("ingest: pipeline must not be nil")

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
