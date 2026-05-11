package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubeboiii/ims/internal/model"
)

// TestSubmit_AcceptUntilFull confirms a non-blocking submit returns true
// exactly `capacity` times when no workers are draining, then flips to
// false forever after. This is the core backpressure contract (FR-1.5).
func TestSubmit_AcceptUntilFull(t *testing.T) {
	p := New(Config{Capacity: 4, Workers: 0, ShutdownTimeout: time.Second},
		func(ctx context.Context, _ model.Signal) error { return nil })
	// NOTE: Workers=0 means no consumers, so the queue truly fills.

	for i := 0; i < 4; i++ {
		if !p.Submit(model.Signal{}) {
			t.Fatalf("submit %d should have been accepted", i)
		}
	}
	if p.Submit(model.Signal{}) {
		t.Fatal("5th submit should have been dropped")
	}
	s := p.Stats()
	if s.Accepted != 4 || s.Dropped != 1 {
		t.Fatalf("counts wrong: %+v", s)
	}
}

// TestPipeline_EveryAcceptedIsProcessed: under concurrent submission,
// every accept must eventually be processed (no silent loss between
// channel receive and processor invocation). We use a generous queue so
// accepts dominate, but the invariant — Accepted == Processed at Stop —
// must hold even if some submits are dropped.
func TestPipeline_EveryAcceptedIsProcessed(t *testing.T) {
	var processed atomic.Uint64
	p := New(Config{Capacity: 8192, Workers: 4, ShutdownTimeout: time.Second},
		func(ctx context.Context, _ model.Signal) error {
			processed.Add(1)
			return nil
		})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p.Start(ctx)

	const N = 5000
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < N/10; j++ {
				p.Submit(model.Signal{})
			}
		}()
	}
	wg.Wait()
	p.Stop()

	s := p.Stats()
	if uint64(processed.Load()) != s.Accepted {
		t.Fatalf("processed=%d, accepted=%d (stats=%+v)", processed.Load(), s.Accepted, s)
	}
	if s.Accepted+s.Dropped != N {
		t.Fatalf("accept+drop=%d, want %d", s.Accepted+s.Dropped, N)
	}
}

// TestPipeline_AcceptedPlusDroppedEqualsTotal — the invariant that proves
// we never double-count nor silently lose. Use a tiny capacity so dropping
// is common.
func TestPipeline_AcceptedPlusDroppedEqualsTotal(t *testing.T) {
	p := New(Config{Capacity: 8, Workers: 2, ShutdownTimeout: time.Second},
		func(ctx context.Context, _ model.Signal) error {
			// add a teensy delay so the queue actually saturates
			time.Sleep(50 * time.Microsecond)
			return nil
		})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	p.Start(ctx)

	const total = 1000
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < total/8; j++ {
				p.Submit(model.Signal{})
			}
		}()
	}
	wg.Wait()
	p.Stop()

	s := p.Stats()
	if s.Accepted+s.Dropped != total {
		t.Fatalf("accept+drop=%d, want %d (stats=%+v)", s.Accepted+s.Dropped, total, s)
	}
}

// TestPipeline_StopIsIdempotent: defensive — main.go's signal handler may
// call Stop more than once if shutdown is messy.
func TestPipeline_StopIsIdempotent(t *testing.T) {
	p := New(Config{Capacity: 4, Workers: 1, ShutdownTimeout: 100 * time.Millisecond},
		func(ctx context.Context, _ model.Signal) error { return nil })
	p.Start(context.Background())
	p.Stop()
	p.Stop() // must not panic on double-close
}
