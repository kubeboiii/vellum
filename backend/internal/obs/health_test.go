package obs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kubeboiii/ims/internal/model"
	"github.com/kubeboiii/ims/internal/pipeline"
)

func newPipe(t *testing.T, cap int) *pipeline.Pipeline {
	t.Helper()
	p := pipeline.New(pipeline.Config{Capacity: cap, Workers: 0, ShutdownTimeout: time.Second},
		func(ctx context.Context, _ model.Signal) error { return nil })
	t.Cleanup(p.Stop)
	return p
}

func TestHealth_HealthyByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	h := NewHealth(p)
	r := gin.New()
	r.GET("/health", h.Handler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if body["status"] != "healthy" {
		t.Fatalf("want healthy, got %v", body["status"])
	}
	if _, ok := body["dependencies"]; !ok {
		t.Fatal("response missing `dependencies` key (Phase 3 will fill it)")
	}
}

func TestHealth_DegradedWhenQueueAbove95Pct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	// Submit 10 onto a capacity-10 queue with 0 workers → 100% full → degraded.
	for i := 0; i < 10; i++ {
		p.Submit(model.Signal{})
	}
	h := NewHealth(p)
	r := gin.New()
	r.GET("/health", h.Handler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when queue saturated, got %d: %s", w.Code, w.Body.String())
	}
}
