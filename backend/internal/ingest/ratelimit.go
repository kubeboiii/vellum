package ingest

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// RateLimiter enforces FR-1.6 in 00-master-prd: a token bucket per source.
// "Source" is the client IP for HTTP; Phase 5's gRPC interceptor will key
// on peer instead but share this same store.
//
// Why this design (not a sliding-window log, not Redis-backed):
//
//   - Token bucket maps cleanly to "1000 req/sec sustained, 2000 burst"
//     which is the FR-1.6 default. golang.org/x/time/rate is the standard
//     Go implementation.
//   - Per-process in-memory state is fine for v1 because we deploy one
//     ingestion replica (01-arch §3.1). If we scaled out, Redis-backed
//     limiting would be the upgrade; the call site stays the same.
//   - Buckets are stored in a map guarded by a single RWMutex. With the
//     standard library's rate.Limiter being lock-free internally, the
//     only contention is on first-time-seen IPs (write lock) — read path
//     is fast enough to handle 10K/sec across a handful of IPs.
//   - A sweeper evicts buckets that haven't been touched for `idle`
//     seconds, so we don't leak memory on the long tail of unique IPs.
type RateLimiter struct {
	rps   rate.Limit
	burst int
	idle  time.Duration

	mu      sync.RWMutex
	buckets map[string]*bucket
}

// bucket holds a per-source token bucket plus a monotonic "last seen"
// timestamp consulted by the sweeper. lastSeenNano is a UnixNano stored
// atomically so the hot path can update it without taking the map mutex.
type bucket struct {
	lim          *rate.Limiter
	lastSeenNano atomic.Int64
}

// NewRateLimiter builds a limiter with the given per-source rps + burst.
// `idle` is the inactivity window after which a bucket is evicted by the
// sweeper. Pass `0` to disable eviction.
func NewRateLimiter(rps float64, burst int, idle time.Duration) *RateLimiter {
	return &RateLimiter{
		rps:     rate.Limit(rps),
		burst:   burst,
		idle:    idle,
		buckets: make(map[string]*bucket),
	}
}

// allow looks up (or creates) the bucket for `key` and returns whether
// this request is allowed. We use Allow() (non-blocking) rather than
// Wait() because the handler must never block — FR-1.4.
func (r *RateLimiter) allow(key string, now time.Time) bool {
	// Fast path: bucket exists. Updating lastSeenNano is an atomic store
	// — sweep races are fine (a slightly stale timestamp at most delays
	// eviction by one sweep cycle).
	r.mu.RLock()
	b, ok := r.buckets[key]
	r.mu.RUnlock()
	if ok {
		b.lastSeenNano.Store(now.UnixNano())
		return b.lim.Allow()
	}
	// Slow path: create the bucket. Re-check under write lock to handle
	// the race where another goroutine created it between RUnlock and Lock.
	r.mu.Lock()
	if b, ok = r.buckets[key]; !ok {
		b = &bucket{lim: rate.NewLimiter(r.rps, r.burst)}
		b.lastSeenNano.Store(now.UnixNano())
		r.buckets[key] = b
	}
	r.mu.Unlock()
	return b.lim.Allow()
}

// Middleware returns a Gin handler that enforces the limit per client IP.
// On reject, returns 429 with a tiny JSON body and `Retry-After: 1`. We
// deliberately keep the body small — the failure mode is hot, the client
// often ignores it, and big bodies just waste bandwidth.
func (r *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.ClientIP()
		if !r.allow(key, time.Now()) {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}

// RunSweeper periodically deletes buckets that have been idle for longer
// than `r.idle`. Call it as `go limiter.RunSweeper(ctx)` from main.go.
// Stops when ctx is cancelled.
func (r *RateLimiter) RunSweeper(ctx interface {
	Done() <-chan struct{}
}) {
	if r.idle <= 0 {
		return
	}
	// Sweep at 1/10 the idle window — frequent enough to keep memory
	// bounded, rare enough to avoid contending with the hot path.
	t := time.NewTicker(r.idle / 10)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			r.sweep(now)
		}
	}
}

func (r *RateLimiter) sweep(now time.Time) {
	cutoffNano := now.Add(-r.idle).UnixNano()
	r.mu.Lock()
	for k, b := range r.buckets {
		if b.lastSeenNano.Load() < cutoffNano {
			delete(r.buckets, k)
		}
	}
	r.mu.Unlock()
}

// Size returns the current number of tracked buckets. Used by tests and
// (Phase 8) the /metrics endpoint.
func (r *RateLimiter) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.buckets)
}
