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

	"github.com/kubeboiii/vellum/internal/model"
	"github.com/kubeboiii/vellum/internal/workflow"
)

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

func TestEngine_HappyPath(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)

	if _, err := engine.Transition(ctx, id, workflow.InvestigatingState{},
		workflow.TransitionContext{Actor: "sre@example.com"}); err != nil {
		t.Fatalf("OPEN->INVESTIGATING: %v", err)
	}

	if _, err := engine.Transition(ctx, id, workflow.ResolvedState{},
		workflow.TransitionContext{Actor: "sre@example.com"}); err != nil {
		t.Fatalf("INVESTIGATING->RESOLVED: %v", err)
	}

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

	wi, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	if wi.Status != model.StatusResolved {
		t.Errorf("status drifted to %s", wi.Status)
	}
}

func TestEngine_RejectsCloseWithoutRCA(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))
	ctx := context.Background()

	id := makeOpenWorkItem(t, repo)
	_, _ = engine.Transition(ctx, id, workflow.InvestigatingState{}, workflow.TransitionContext{})
	_, _ = engine.Transition(ctx, id, workflow.ResolvedState{}, workflow.TransitionContext{})

	_, err := engine.Transition(ctx, id, workflow.ClosedState{}, workflow.TransitionContext{})
	if !errors.Is(err, workflow.ErrMissingRCA) {
		t.Fatalf("want ErrMissingRCA, got %v", err)
	}
	wi, _ := repo.GetByID(ctx, id)
	if wi.Status != model.StatusResolved {
		t.Errorf("status drifted to %s after rejected close", wi.Status)
	}
}

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

func TestEngine_NotFound(t *testing.T) {
	pool := startPostgres(t)
	repo := NewWorkItemRepository(pool)
	engine := workflow.NewEngine(NewWorkflowTxRunner(repo))

	_, err := engine.Transition(context.Background(), uuid.New(), workflow.InvestigatingState{}, workflow.TransitionContext{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
