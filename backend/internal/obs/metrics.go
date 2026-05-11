package obs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kubeboiii/ims/internal/pipeline"
)

// MetricsTicker prints a single structured line every `interval` (FR-8.2 in
// 00-master-prd). The line format matches the sample in 01-architecture
// §11.2 so it's parseable by simple regex / awk.
//
// We compute per-interval rates by sampling the cumulative counters at
// each tick and diffing against the previous sample. This avoids the
// rounding noise of computing rates inside the pipeline itself and keeps
// the pipeline counters as plain monotonic atomics.
type MetricsTicker struct {
	pipe     *pipeline.Pipeline
	interval time.Duration
}

// NewMetricsTicker returns a ticker that reads from `p` every `interval`.
// `interval` defaults to 5s if non-positive.
func NewMetricsTicker(p *pipeline.Pipeline, interval time.Duration) *MetricsTicker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &MetricsTicker{pipe: p, interval: interval}
}

// Run blocks until ctx is cancelled, emitting one log line per tick. Call
// as `go ticker.Run(ctx)` from main.go. On exit, prints one final summary
// so the operator gets closure even on graceful shutdown.
func (m *MetricsTicker) Run(ctx context.Context) {
	t := time.NewTicker(m.interval)
	defer t.Stop()

	prev := m.pipe.Stats()
	prevAt := time.Now()
	for {
		select {
		case <-ctx.Done():
			// Final snapshot so the operator sees the absolute totals.
			s := m.pipe.Stats()
			log.Printf("[metrics-final] accepted=%d processed=%d dropped=%d errors=%d queue=%d/%d",
				s.Accepted, s.Processed, s.Dropped, s.Errors, s.QueueDepth, s.Capacity)
			return
		case now := <-t.C:
			cur := m.pipe.Stats()
			elapsed := now.Sub(prevAt).Seconds()
			if elapsed <= 0 {
				elapsed = 1 // defensive; shouldn't happen
			}
			accRate := float64(cur.Accepted-prev.Accepted) / elapsed
			procRate := float64(cur.Processed-prev.Processed) / elapsed
			errRate := float64(cur.Errors-prev.Errors) / elapsed

			// Single line, key=value, in the format from 01-arch §11.2.
			fmt.Printf("[metrics] accepted=%.0f/s processed=%.0f/s queue=%d/%d errors=%.2f/s total_accepted=%d total_dropped=%d\n",
				accRate, procRate, cur.QueueDepth, cur.Capacity,
				errRate, cur.Accepted, cur.Dropped)

			prev = cur
			prevAt = now
		}
	}
}
