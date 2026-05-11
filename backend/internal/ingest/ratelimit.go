package ingest

import (
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	rps   rate.Limit
	burst int
	idle  time.Duration

	mu      sync.RWMutex
	buckets map[string]*bucket
}

type bucket struct {
	lim          *rate.Limiter
	lastSeenNano atomic.Int64
}

func NewRateLimiter(rps float64, burst int, idle time.Duration) *RateLimiter {
	return &RateLimiter{
		rps:     rate.Limit(rps),
		burst:   burst,
		idle:    idle,
		buckets: make(map[string]*bucket),
	}
}

func (r *RateLimiter) allow(key string, now time.Time) bool {

	r.mu.RLock()
	b, ok := r.buckets[key]
	r.mu.RUnlock()
	if ok {
		b.lastSeenNano.Store(now.UnixNano())
		return b.lim.Allow()
	}

	r.mu.Lock()
	if b, ok = r.buckets[key]; !ok {
		b = &bucket{lim: rate.NewLimiter(r.rps, r.burst)}
		b.lastSeenNano.Store(now.UnixNano())
		r.buckets[key] = b
	}
	r.mu.Unlock()
	return b.lim.Allow()
}

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

func (r *RateLimiter) RunSweeper(ctx interface {
	Done() <-chan struct{}
}) {
	if r.idle <= 0 {
		return
	}

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

func (r *RateLimiter) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.buckets)
}
