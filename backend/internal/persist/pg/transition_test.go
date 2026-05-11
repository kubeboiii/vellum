package pg

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/ims/internal/model"
	"github.com/kubeboiii/ims/internal/workflow"
)

// makeOpenWorkItem inserts a fresh OPEN-state work_item into the DB
// and returns its ID. The seeded signal's timestamp is backdated by
// one hour so MTTR has a measurable value when we close the incident
// at "now" — a real-world incident isn't sub-second.
func makeOpenWorkItem(t *testing.T, repo *WorkItemRepository) uuid.UUID {
	t.Helper()
	wi := model.NewWorkItem(uuid.New(), model.Signal{
		ComponentID:   "X_" + uuid.NewString()[:8],
		ComponentType: model.ComponentCache,
		Severity:      model.SeverityP0,
		Source:        "test",
		Timestamp:     time.Now().UTC().Add(-1 * time.Hour),
	})
	if err := repo.Insert(context.Background(), wi); err != nil {
		t.Fatalf("seed work_item: %v", err)
	}
	return wi.ID
}

// TestEngine_HappyPath: OPEN → INVESTIGATING → RESOLVED → CLOSED with
// a valid RCA. After CLOSED, MTTR is populated, the rca row exists,
// and three state_transition audit rows are recorded.
func TestEngine_HappyPath(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)

	// OPEN -> INVESTIGATING
	if _, err := engine.Transition(ctx, id, workflow.InvestigatingState{},
		workflow.TransitionContext{Actor: "sre@example.com"}); err != nil {
		t.Fatalf("OPEN->INVESTIGATING: %v", err)
	}
	// INVESTIGATING -> RESOLVED
	if _, err := engine.Transition(ctx, id, workflow.ResolvedState{},
		workflow.TransitionContext{Actor: "sre@example.com"}); err != nil {
		t.Fatalf("INVESTIGATING->RESOLVED: %v", err)
	}
	// RESOLVED -> CLOSED with a complete RCA
	rca := &model.RCA{
		RootCauseCategory: model.CategoryInfrastructure,
		FixApplied:        strings.Repeat("a", 30),
		PreventionSteps:   strings.Repeat("b", 30),
		SubmittedBy:       "sre@example.com",
	}
	closedWI, closedRCA, err := engine.CloseWithRCA(ctx, id, rca, "sre@example.com")
	if err != nil {
		t.Fatalf("CloseWithRCA: %v", err)
	}
	if closedWI.Status != model.StatusClosed {
		t.Errorf("status: want CLOSED, got %s", closedWI.Status)
	}
	if closedWI.MTTRSeconds == nil || *closedWI.MTTRSeconds <= 0 {
		t.Errorf("MTTR should be set, got %v", closedWI.MTTRSeconds)
	}
	if closedRCA.ID == uuid.Nil {
		t.Error("RCA.ID should be set")
	}

	// Verify state_transitions has 3 rows.
	var transCount int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM state_transitions WHERE work_item_id = $1`, id,
	).Scan(&transCount); err != nil {
		t.Fatalf("count transitions: %v", err)
	}
	if transCount != 3 {
		t.Errorf("expected 3 state_transition rows, got %d", transCount)
	}
}

// TestEngine_RejectsBackward: trying RESOLVED → OPEN returns
// ErrInvalidTransition and the row stays at RESOLVED.
func TestEngine_RejectsBackward(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)
	_, _ = engine.Transition(ctx, id, workflow.InvestigatingState{}, workflow.TransitionContext{})
	_, _ = engine.Transition(ctx, id, workflow.ResolvedState{}, workflow.TransitionContext{})

	_, err := engine.Transition(ctx, id, workflow.OpenState{}, workflow.TransitionContext{})
	if !errors.Is(err, workflow.ErrInvalidTransition) {
		t.Fatalf("want ErrInvalidTransition, got %v", err)
	}
	// Verify the row is still RESOLVED.
	wi, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if wi.Status != model.StatusResolved {
		t.Errorf("status drifted to %s", wi.Status)
	}
}

// TestEngine_RejectsCloseWithoutRCA: OPEN→INVESTIGATING→RESOLVED→CLOSE
// without an RCA returns ErrMissingRCA. The DB row stays RESOLVED.
func TestEngine_RejectsCloseWithoutRCA(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)
	_, _ = engine.Transition(ctx, id, workflow.InvestigatingState{}, workflow.TransitionContext{})
	_, _ = engine.Transition(ctx, id, workflow.ResolvedState{}, workflow.TransitionContext{})

	// Plain Transition into CLOSED with no RCA in the ctx.
	_, err := engine.Transition(ctx, id, workflow.ClosedState{}, workflow.TransitionContext{})
	if !errors.Is(err, workflow.ErrMissingRCA) {
		t.Fatalf("want ErrMissingRCA, got %v", err)
	}
	wi, _ := repo.GetByID(ctx, id)
	if wi.Status != model.StatusResolved {
		t.Errorf("status drifted to %s after rejected close", wi.Status)
	}
}

// TestEngine_RejectsCloseWithIncompleteRCA: a present but invalid RCA
// returns ErrIncompleteRCA and exposes the field errors.
func TestEngine_RejectsCloseWithIncompleteRCA(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)
	_, _ = engine.Transition(ctx, id, workflow.InvestigatingState{}, workflow.TransitionContext{})
	_, _ = engine.Transition(ctx, id, workflow.ResolvedState{}, workflow.TransitionContext{})

	bad := &model.RCA{
		RootCauseCategory: model.CategoryInfrastructure,
		FixApplied:        "too short",
		PreventionSteps:   "also short",
		SubmittedBy:       "test@example.com",
	}
	_, _, err := engine.CloseWithRCA(ctx, id, bad, "test@example.com")
	if !errors.Is(err, workflow.ErrIncompleteRCA) {
		t.Fatalf("want ErrIncompleteRCA, got %v", err)
	}
	var ire *workflow.IncompleteRCAError
	if !errors.As(err, &ire) || len(ire.Fields) == 0 {
		t.Fatalf("expected IncompleteRCAError with fields, got %v", err)
	}
}

// TestEngine_ConcurrentClose_ExactlyOneWins: R2 in 00-master-prd —
// "concurrency bugs hide in tests" — the canonical concurrency test
// for this project. Two goroutines try to close the same WI with the
// same valid RCA. Exactly one must succeed; the other gets either
// ErrInvalidTransition (because the loser sees status=CLOSED) or a
// DB-level unique-constraint failure (the rca row already exists).
// Either is correct behaviour.
func TestEngine_ConcurrentClose_ExactlyOneWins(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)
	_, _ = engine.Transition(ctx, id, workflow.InvestigatingState{}, workflow.TransitionContext{})
	_, _ = engine.Transition(ctx, id, workflow.ResolvedState{}, workflow.TransitionContext{})

	rcaFor := func() *model.RCA {
		return &model.RCA{
			RootCauseCategory: model.CategoryHumanError,
			FixApplied:        strings.Repeat("x", 30),
			PreventionSteps:   strings.Repeat("y", 30),
			SubmittedBy:       "race@example.com",
		}
	}

	var successes atomic.Int64
	var failures atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, err := engine.CloseWithRCA(ctx, id, rcaFor(), "race@example.com")
			if err == nil {
				successes.Add(1)
			} else {
				failures.Add(1)
				t.Logf("loser err: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Errorf("want exactly 1 success, got %d (failures=%d)", successes.Load(), failures.Load())
	}

	// Verify final state: WI is CLOSED, exactly one rca row, exactly
	// one extra state_transition (RESOLVED→CLOSED).
	wi, _ := repo.GetByID(ctx, id)
	if wi.Status != model.StatusClosed {
		t.Errorf("final status: want CLOSED, got %s", wi.Status)
	}
	var rcaCount int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM rca WHERE work_item_id = $1`, id).Scan(&rcaCount)
	if rcaCount != 1 {
		t.Errorf("rca count: want 1, got %d", rcaCount)
	}
}

// TestEngine_NotFound: transitioning a non-existent WI ID returns
// ErrNotFound (mapped to 404 by the API handler).
func TestEngine_NotFound(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))

	_, err := engine.Transition(context.Background(), uuid.New(), workflow.InvestigatingState{}, workflow.TransitionContext{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
