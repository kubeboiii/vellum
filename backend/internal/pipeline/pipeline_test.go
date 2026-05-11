package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubeboiii/vellum/internal/model"
)

func TestSubmit_AcceptUntilFull(t *testing.T) {
	p := New(Config{Capacity: 4, Workers: 0, ShutdownTimeout: time.Second},
		func(ctx context.Context, _ model.Signal) error { return nil })

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

func TestPipeline_AcceptedPlusDroppedEqualsTotal(t *testing.T) {
	p := New(Config{Capacity: 8, Workers: 2, ShutdownTimeout: time.Second},
		func(ctx context.Context, _ model.Signal) error {

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

func TestPipeline_StopIsIdempotent(t *testing.T) {
	p := New(Config{Capacity: 4, Workers: 1, ShutdownTimeout: 100 * time.Millisecond},
		func(ctx context.Context, _ model.Signal) error { return nil })
	p.Start(context.Background())
	p.Stop()
	p.Stop()
}
