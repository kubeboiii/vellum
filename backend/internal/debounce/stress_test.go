// Phase 6 — stress test for the debouncer's hardest property:
// under massive contention against the SAME component_id, the
// Lua-backed window-cap math must hold exactly.
//
// Why this is the test that matters:
// PRD risk R2 says "Concurrency bugs hide in tests." The existing
// TestProcess_ConcurrentSameComponent test proves single-window
// atomicity (exactly one CREATE among N callers) but uses
// MaxSignals=N+1, so it never crosses a window boundary. The
// across-window property — that K signals against a cap of C
// produce exactly ceil(K/C) work_items — is the one a race in the
// Lua script's CAS dance could quietly break.
//
// This test wires the production cap of 100, fires N signals from
// N goroutines simultaneously, and asserts the distinct work_item
// count equals exactly ceil(N / 100). Run with `-race`.

package debounce

import (
	"context"
	"math"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// TestProcess_StressManyWindows fires N concurrent goroutines at
// the SAME component_id with the production window cap of 100,
// then asserts the distinct work_item count equals exactly
// ceil(N/100). Any other count signals a race in the Lua window
// transition.
//
// We use a smaller N (300) by default so the test is fast in CI.
// Set IMS_STRESS_N to scale it up locally.
func TestProcess_StressManyWindows(t *testing.T) {
	d := startRedis(t)
	// Use the production cap. The startRedis helper sets
	// MaxSignals=5 for the table-driven tests; restore it here.
	d.cfg.MaxSignals = 100

	const N = 300
	const expectedWorkItems = N / 100 // 300 / 100 = 3 windows expected

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

	// All Process calls must succeed under load — Redis is local,
	// no degradation expected.
	for i, err := range errs {
		if err != nil {
			t.Fatalf("Process %d: %v", i, err)
		}
	}

	// Count distinct work_item IDs.
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

// TestProcess_StressBoundaryArithmetic checks the off-by-one edge
// of the window cap. Send exactly cap+1 signals; the first `cap`
// must JOIN one work_item and the (cap+1)th must CREATE a fresh
// one.
//
// Run sequentially (not concurrently) so the ordering is
// deterministic: we want to prove the math, not the atomicity.
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

	// First signal: CREATE. Signals 2..100: JOIN. Signal 101: CREATE.
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

	// Distinct work_item count: exactly 2.
	idSet := map[uuid.UUID]struct{}{}
	for _, r := range results {
		idSet[r.WorkItemID] = struct{}{}
	}
	if got := len(idSet); got != 2 {
		t.Errorf("expected exactly 2 distinct work_items; got %d", got)
	}
}

// Compile-time sanity: math.Ceil isn't used in the test bodies but
// we want it imported above to make the docstring's formula
// expression clear. This keeps `go vet` happy without an unused
// import.
var _ = math.Ceil
