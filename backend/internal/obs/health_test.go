package obs

import (
	"context"
	"encoding/json"
	"errors"
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

// stubPinger lets tests drive the Pinger.Ping return value without
// spinning up a real DB. nameFn parameterises so multiple stubs can
// coexist.
type stubPinger struct {
	name string
	err  error
}

func (s stubPinger) Name() string                 { return s.name }
func (s stubPinger) Ping(_ context.Context) error { return s.err }

func TestHealth_HealthyByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	h := NewHealth(p, HealthConfig{})
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
		t.Fatal("response missing `dependencies` key")
	}
}

func TestHealth_DegradedWhenQueueAbove95Pct(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	// Submit 10 onto a capacity-10 queue with 0 workers → 100% full → degraded.
	for i := 0; i < 10; i++ {
		p.Submit(model.Signal{})
	}
	h := NewHealth(p, HealthConfig{})
	r := gin.New()
	r.GET("/health", h.Handler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503 when queue saturated, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHealth_AllDepsUp: registered pingers all healthy → 200 healthy.
func TestHealth_AllDepsUp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	h := NewHealth(p, HealthConfig{
		Deps: []Pinger{
			stubPinger{name: "postgres"},
			stubPinger{name: "mongo"},
			stubPinger{name: "redis"},
		},
	})
	r := gin.New()
	r.GET("/health", h.Handler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	var body struct {
		Status       string                    `json:"status"`
		Dependencies map[string]map[string]any `json:"dependencies"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Status != "healthy" {
		t.Errorf("status: want healthy, got %q", body.Status)
	}
	for _, name := range []string{"postgres", "mongo", "redis"} {
		dep := body.Dependencies[name]
		if dep["status"] != "up" {
			t.Errorf("%s: want up, got %v", name, dep["status"])
		}
	}
}

// TestHealth_CriticalDepDown_Returns503: Postgres down → 503.
func TestHealth_CriticalDepDown_Returns503(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	h := NewHealth(p, HealthConfig{
		Deps: []Pinger{
			stubPinger{name: "postgres", err: errors.New("conn refused")},
			stubPinger{name: "mongo"},
		},
	})
	r := gin.New()
	r.GET("/health", h.Handler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("critical down: want 503, got %d", w.Code)
	}
}

// TestHealth_NonCriticalDepDown_Returns200Degraded: Redis (non-critical)
// down → status="degraded" but HTTP 200. The system can still process
// signals via the FR-3.6 fallback.
func TestHealth_NonCriticalDepDown_Returns200Degraded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	p := newPipe(t, 10)
	h := NewHealth(p, HealthConfig{
		Deps: []Pinger{
			stubPinger{name: "postgres"},
			stubPinger{name: "mongo"},
			stubPinger{name: "redis", err: errors.New("conn refused")},
		},
	})
	r := gin.New()
	r.GET("/health", h.Handler())

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("non-critical down: want 200, got %d", w.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Status != "degraded" {
		t.Errorf("status: want degraded, got %q", body.Status)
	}
}
