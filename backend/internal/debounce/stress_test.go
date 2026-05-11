package debounce

import (
	"context"
	"math"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestProcess_StressManyWindows(t *testing.T) {
	d := startRedis(t)

	d.cfg.MaxSignals = 100

	const N = 300
	const expectedWorkItems = N / 100

	ctx := context.Background()
	results := make([]Result, N)
	errs := make([]error, N)
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := d.Process(ctx, "STRESS_01")
			results[i] = r
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("Process %d: %v", i, err)
		}
	}

	idSet := map[uuid.UUID]struct{}{}
	var creates int
	for _, r := range results {
		idSet[r.WorkItemID] = struct{}{}
		if r.Action == ActionCreated {
			creates++
		}
	}

	if got := len(idSet); got != expectedWorkItems {
		t.Errorf(
			"expected exactly %d distinct work_items (ceil(%d/%d)); got %d",
			expectedWorkItems, N, d.cfg.MaxSignals, got,
		)
	}
	if creates != expectedWorkItems {
		t.Errorf(
			"expected exactly %d CREATED actions (one per new window); got %d",
			expectedWorkItems, creates,
		)
	}
}

func TestProcess_StressBoundaryArithmetic(t *testing.T) {
	d := startRedis(t)
	d.cfg.MaxSignals = 100

	ctx := context.Background()
	const total = 101
	results := make([]Result, total)
	for i := 0; i < total; i++ {
		r, err := d.Process(ctx, "BOUNDARY_01")
		if err != nil {
			t.Fatalf("Process %d: %v", i, err)
		}
		results[i] = r
	}

	if results[0].Action != ActionCreated {
		t.Errorf("signal 0: want CREATED, got %s", results[0].Action)
	}
	for i := 1; i < 100; i++ {
		if results[i].Action != ActionJoined {
			t.Errorf("signal %d: want JOINED, got %s", i, results[i].Action)
		}
	}
	if results[100].Action != ActionCreated {
		t.Errorf("signal 100 (the cap+1th): want CREATED (new window), got %s", results[100].Action)
	}

	idSet := map[uuid.UUID]struct{}{}
	for _, r := range results {
		idSet[r.WorkItemID] = struct{}{}
	}
	if got := len(idSet); got != 2 {
		t.Errorf("expected exactly 2 distinct work_items; got %d", got)
	}
}

var _ = math.Ceil
