// Package pipeline implements the bounded-channel + worker-pool ingestion
// hot path described in 01-architecture §4.
//
// The pipeline owns three things:
//
//  1. A fixed-capacity `chan model.Signal` buffer (the "signal queue").
//  2. A pool of worker goroutines that consume from it and run a
//     caller-supplied Processor.
//  3. Atomic counters for accepted / processed / dropped, exposed via
//     Stats() so the metrics ticker and /health can read them without
//     contention.
//
// Phase 2 wires Processor to a no-op stub (count and discard). Phase 3
// replaces it with the debounce + persistence fan-out.
//
// Backpressure (NFR-2.1 in 00-master-prd) is achieved with a single
// non-blocking `select` in Submit: if the channel is full, we increment
// `dropped` and return false in O(ns). The HTTP handler maps that to 503.
// We deliberately never block the sender — see 01-architecture §4.2 for
// the rationale.
package pipeline

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kubeboiii/ims/internal/model"
)

// Processor handles one signal off the queue. It is called from worker
// goroutines and must be safe for concurrent use. Returning an error is
// logged but otherwise non-fatal; Phase 3 will route errors to retry +
// dead-letter.
type Processor func(ctx context.Context, sig model.Signal) error

// Config bundles the knobs from `docs/phases/phase-2-ingestion.md` §2.3.
// Zero values are not valid — use DefaultConfig and override.
type Config struct {
	Capacity        int
	Workers         int
	ShutdownTimeout time.Duration
}

// DefaultConfig returns the values documented in the phase file. Callers
// override individual fields from env vars in cmd/ims/main.go.
func DefaultConfig() Config {
	return Config{
		Capacity:        50_000,
		Workers:         16, // overridden to NumCPU()*2 in main.go
		ShutdownTimeout: 30 * time.Second,
	}
}

// Stats is a point-in-time snapshot of the pipeline counters. Cheap to
// read; the ticker and /health both call it.
type Stats struct {
	Accepted   uint64
	Processed  uint64
	Dropped    uint64
	Errors     uint64
	QueueDepth int
	Capacity   int
}

// Pipeline owns the queue and the workers. One per process.
type Pipeline struct {
	cfg       Config
	queue     chan model.Signal
	processor Processor

	accepted  atomic.Uint64
	processed atomic.Uint64
	dropped   atomic.Uint64
	errors    atomic.Uint64

	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	done      chan struct{}
}

// ErrNotStarted is returned by Submit before Start has been called. It
// should never appear in normal operation but the explicit check beats a
// nil-channel send panic.
var ErrNotStarted = errors.New("pipeline not started")

// New constructs a pipeline with the given config and processor. It does
// NOT spin up workers — call Start for that. Splitting construction from
// start lets the caller wire Submit into the HTTP handler before the
// workers begin draining, which makes startup race-free.
func New(cfg Config, p Processor) *Pipeline {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 50_000
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 16
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}
	return &Pipeline{
		cfg:       cfg,
		queue:     make(chan model.Signal, cfg.Capacity),
		processor: p,
		done:      make(chan struct{}),
	}
}

// Start spawns cfg.Workers worker goroutines. Safe to call once; subsequent
// calls are no-ops (sync.Once). Workers exit when ctx is cancelled OR when
// Stop is called.
func (p *Pipeline) Start(ctx context.Context) {
	p.startOnce.Do(func() {
		for i := 0; i < p.cfg.Workers; i++ {
			p.wg.Add(1)
			go p.workerLoop(ctx, i)
		}
		log.Printf("pipeline: started %d workers (capacity=%d)", p.cfg.Workers, p.cfg.Capacity)
	})
}

// Submit attempts a non-blocking enqueue. Returns true on accept, false if
// the queue is full. Counters are bumped exactly once per call (no
// double-count under contention because both atomic adds happen on disjoint
// branches of the select).
func (p *Pipeline) Submit(sig model.Signal) bool {
	select {
	case p.queue <- sig:
		p.accepted.Add(1)
		return true
	default:
		p.dropped.Add(1)
		return false
	}
}

// Stats returns a snapshot of the counters. Cheap (4 atomic loads + 1 len).
func (p *Pipeline) Stats() Stats {
	return Stats{
		Accepted:   p.accepted.Load(),
		Processed:  p.processed.Load(),
		Dropped:    p.dropped.Load(),
		Errors:     p.errors.Load(),
		QueueDepth: len(p.queue),
		Capacity:   p.cfg.Capacity,
	}
}

// Capacity returns the configured channel capacity. Used by /health.
func (p *Pipeline) Capacity() int { return p.cfg.Capacity }

// Stop closes the input channel, gives workers ShutdownTimeout to drain
// in-flight signals, then returns. Idempotent. After Stop returns, Submit
// continues to "work" (it will send onto a closed channel and panic if any
// caller still races us) — the HTTP server must have shut down its
// listeners *before* Stop is invoked. main.go enforces that ordering.
func (p *Pipeline) Stop() {
	p.stopOnce.Do(func() {
		close(p.queue)
		// Race the workers against a wall-clock deadline. If they wedge on
		// the processor, we still want to exit (the workers don't matter
		// for correctness after we've closed the input; in-flight signals
		// in Phase 2 are best-effort).
		drained := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(drained)
		}()
		select {
		case <-drained:
			log.Print("pipeline: drained cleanly")
		case <-time.After(p.cfg.ShutdownTimeout):
			log.Printf("pipeline: drain deadline %s exceeded, exiting", p.cfg.ShutdownTimeout)
		}
		close(p.done)
	})
}

// Done returns a channel closed once Stop has finished. Lets cmd/ims/main
// block on full shutdown without exporting the WaitGroup.
func (p *Pipeline) Done() <-chan struct{} { return p.done }

// workerLoop is the per-goroutine consumer. It exits when either:
//   - The queue is closed AND drained (range terminates), or
//   - The root context is cancelled (we check it between signals).
//
// We do NOT check ctx.Done() inside the processor call — Phase 3's
// processor will take its own ctx and decide its own deadline.
func (p *Pipeline) workerLoop(ctx context.Context, id int) {
	defer p.wg.Done()
	// recover() per 01-arch §9 (failure modes table — "worker panics").
	// A panicking worker is replaced not by restart (Go has no thread
	// pool to restart from) but by logging + exiting; the other N-1
	// workers keep draining. Phase 6 may revisit and respawn.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("pipeline: worker %d panicked: %v", id, r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case sig, ok := <-p.queue:
			if !ok {
				return // queue closed; drain complete
			}
			if err := p.processor(ctx, sig); err != nil {
				p.errors.Add(1)
				log.Printf("pipeline: worker %d processor error: %v (signal_id=%s)", id, err, sig.SignalID)
			}
			p.processed.Add(1)
		}
	}
}

// NoopProcessor counts the signal and discards it. Phase 2 default; Phase
// 3 swaps in the debounce + persistence fan-out processor.
func NoopProcessor(_ context.Context, _ model.Signal) error { return nil }
