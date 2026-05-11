package obs

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/kubeboiii/vellum/internal/pipeline"
)

type MetricsTicker struct {
	pipe     *pipeline.Pipeline
	interval time.Duration
}

func NewMetricsTicker(p *pipeline.Pipeline, interval time.Duration) *MetricsTicker {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &MetricsTicker{pipe: p, interval: interval}
}

func (m *MetricsTicker) Run(ctx context.Context) {
	t := time.NewTicker(m.interval)
	defer t.Stop()

	prev := m.pipe.Stats()
	prevAt := time.Now()
	for {
		select {
		case <-ctx.Done():

			s := m.pipe.Stats()
			log.Printf("[metrics-final] accepted=%d processed=%d dropped=%d errors=%d queue=%d/%d",
				s.Accepted, s.Processed, s.Dropped, s.Errors, s.QueueDepth, s.Capacity)
			return
		case now := <-t.C:
			cur := m.pipe.Stats()
			elapsed := now.Sub(prevAt).Seconds()
			if elapsed <= 0 {
				elapsed = 1
			}
			accRate := float64(cur.Accepted-prev.Accepted) / elapsed
			procRate := float64(cur.Processed-prev.Processed) / elapsed
			errRate := float64(cur.Errors-prev.Errors) / elapsed

			fmt.Printf("[metrics] accepted=%.0f/s processed=%.0f/s queue=%d/%d errors=%.2f/s total_accepted=%d total_dropped=%d\n",
				accRate, procRate, cur.QueueDepth, cur.Capacity,
				errRate, cur.Accepted, cur.Dropped)

			prev = cur
			prevAt = now
		}
	}
}
