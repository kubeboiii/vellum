package ingest

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestRateLimiter_AllowsBurstThenRejects: with rps=1 and burst=3, the
// first 3 hits succeed and the 4th gets 429. Sleeping past the refill
// frees the next token. This exercises the bucket semantics and the
// HTTP wiring together.
func TestRateLimiter_AllowsBurstThenRejects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(1.0, 3, time.Minute)
	r := gin.New()
	r.Use(limiter.Middleware())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	hit := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		r.ServeHTTP(w, req)
		return w.Code
	}

	for i := 0; i < 3; i++ {
		if got := hit(); got != http.StatusOK {
			t.Fatalf("hit %d: want 200, got %d", i, got)
		}
	}
	if got := hit(); got != http.StatusTooManyRequests {
		t.Fatalf("4th hit: want 429, got %d", got)
	}
	// Wait long enough for one token to refill (rate=1/sec means ~1s).
	time.Sleep(1100 * time.Millisecond)
	if got := hit(); got != http.StatusOK {
		t.Fatalf("after refill: want 200, got %d", got)
	}
}

// TestRateLimiter_IsolatesSources: a chatty IP can't starve a quiet one.
// FR-1.6 explicitly requires per-source.
func TestRateLimiter_IsolatesSources(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := NewRateLimiter(1.0, 1, time.Minute)
	r := gin.New()
	r.Use(limiter.Middleware())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	hit := func(ip string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.RemoteAddr = ip + ":1234"
		r.ServeHTTP(w, req)
		return w.Code
	}

	// Burn IP A's budget.
	if hit("10.0.0.1") != http.StatusOK {
		t.Fatal("first A: should be 200")
	}
	if hit("10.0.0.1") != http.StatusTooManyRequests {
		t.Fatal("second A: should be 429")
	}
	// IP B should be unaffected.
	if hit("10.0.0.2") != http.StatusOK {
		t.Fatal("first B: should be 200")
	}
}

// TestRateLimiter_ConcurrentBucketCreation: hammering with new IPs from
// many goroutines must not double-create a bucket or race on the map.
// Critical under `-race`.
func TestRateLimiter_ConcurrentBucketCreation(t *testing.T) {
	limiter := NewRateLimiter(1000, 1000, time.Minute)

	var wg sync.WaitGroup
	const N = 100
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			limiter.allow("10.0.0.1", time.Now()) // all the same IP
		}(i)
	}
	wg.Wait()
	if got := limiter.Size(); got != 1 {
		t.Fatalf("want 1 bucket, got %d", got)
	}
}
