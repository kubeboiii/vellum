package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/vellum/internal/model"
)

func completeRCA() *model.RCA {
	r := &model.RCA{
		WorkItemID:        uuid.New(),
		IncidentStart:     time.Date(2026, 5, 12, 3, 0, 0, 0, time.UTC),
		IncidentEnd:       time.Date(2026, 5, 12, 4, 0, 0, 0, time.UTC),
		RootCauseCategory: model.CategoryInfrastructure,
		FixApplied:        strings.Repeat("a", 30),
		PreventionSteps:   strings.Repeat("b", 30),
		SubmittedBy:       "sre@example.com",
	}
	return r
}

func TestTransitions_HappyPath(t *testing.T) {
	open := OpenState{}
	if err := open.CanTransitionTo(InvestigatingState{}, TransitionContext{}); err != nil {
		t.Errorf("OPEN -> INVESTIGATING: %v", err)
	}

	inv := InvestigatingState{}
	if err := inv.CanTransitionTo(ResolvedState{}, TransitionContext{}); err != nil {
		t.Errorf("INVESTIGATING -> RESOLVED: %v", err)
	}

	res := ResolvedState{}
	tctx := TransitionContext{RCA: completeRCA()}
	if err := res.CanTransitionTo(ClosedState{}, tctx); err != nil {
		t.Errorf("RESOLVED -> CLOSED (with valid RCA): %v", err)
	}
}

func TestTransitions_BackwardRejected(t *testing.T) {
	cases := []struct {
		name string
		from State
		to   State
	}{
		{"INVESTIGATING -> OPEN", InvestigatingState{}, OpenState{}},
		{"RESOLVED -> INVESTIGATING", ResolvedState{}, InvestigatingState{}},
		{"RESOLVED -> OPEN", ResolvedState{}, OpenState{}},
		{"CLOSED -> RESOLVED", ClosedState{}, ResolvedState{}},
		{"CLOSED -> INVESTIGATING", ClosedState{}, InvestigatingState{}},
		{"CLOSED -> OPEN", ClosedState{}, OpenState{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.CanTransitionTo(tc.to, TransitionContext{RCA: completeRCA()})
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("want ErrInvalidTransition, got %v", err)
			}
		})
	}
}

func TestTransitions_SkipRejected(t *testing.T) {
	cases := []struct {
		name string
		from State
		to   State
	}{
		{"OPEN -> RESOLVED", OpenState{}, ResolvedState{}},
		{"OPEN -> CLOSED", OpenState{}, ClosedState{}},
		{"INVESTIGATING -> CLOSED", InvestigatingState{}, ClosedState{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.from.CanTransitionTo(tc.to, TransitionContext{RCA: completeRCA()})
			if !errors.Is(err, ErrInvalidTransition) {
				t.Errorf("want ErrInvalidTransition, got %v", err)
			}
		})
	}
}

func TestRESOLVED_to_CLOSED_NoRCA(t *testing.T) {
	err := ResolvedState{}.CanTransitionTo(ClosedState{}, TransitionContext{RCA: nil})
	if !errors.Is(err, ErrMissingRCA) {
		t.Fatalf("want ErrMissingRCA, got %v", err)
	}
}

func TestRESOLVED_to_CLOSED_IncompleteRCA(t *testing.T) {
	bad := completeRCA()
	bad.FixApplied = "too short"
	bad.RootCauseCategory = "WAT"

	err := ResolvedState{}.CanTransitionTo(ClosedState{}, TransitionContext{RCA: bad})
	if !errors.Is(err, ErrIncompleteRCA) {
		t.Fatalf("want ErrIncompleteRCA, got %v", err)
	}
	var ire *IncompleteRCAError
	if !errors.As(err, &ire) {
		t.Fatalf("expected error to be *IncompleteRCAError")
	}
	if len(ire.Fields) != 2 {
		t.Errorf("expected 2 field errors, got %d: %v", len(ire.Fields), ire.Fields)
	}
}

func TestClosedState_OnEnter_ComputesMTTR(t *testing.T) {
	wi := &model.WorkItem{}
	rca := completeRCA()
	err := ClosedState{}.OnEnter(context.Background(), wi, TransitionContext{RCA: rca})
	if err != nil {
		t.Fatalf("OnEnter: %v", err)
	}
	if wi.MTTRSeconds == nil || *wi.MTTRSeconds != 3600 {
		t.Errorf("MTTR: want 3600, got %v", wi.MTTRSeconds)
	}
	if wi.ClosedAt == nil {
		t.Error("ClosedAt should be set")
	}
	if wi.IncidentStart == nil || !wi.IncidentStart.Equal(rca.IncidentStart) {
		t.Errorf("IncidentStart not copied from RCA")
	}
	if wi.IncidentEnd == nil || !wi.IncidentEnd.Equal(rca.IncidentEnd) {
		t.Errorf("IncidentEnd not copied from RCA")
	}
}

func TestClosedState_OnEnter_RejectsMissingRCA(t *testing.T) {
	wi := &model.WorkItem{}
	err := ClosedState{}.OnEnter(context.Background(), wi, TransitionContext{})
	if err == nil {
		t.Fatal("OnEnter must reject nil RCA")
	}
}

func TestFromStatus_KnownValues(t *testing.T) {
	cases := []struct {
		status model.Status
		want   model.Status
	}{
		{model.StatusOpen, model.StatusOpen},
		{model.StatusInvestigating, model.StatusInvestigating},
		{model.StatusResolved, model.StatusResolved},
		{model.StatusClosed, model.StatusClosed},
	}
	for _, tc := range cases {
		s, err := FromStatus(tc.status)
		if err != nil {
			t.Errorf("FromStatus(%q): %v", tc.status, err)
			continue
		}
		if s.Name() != tc.want {
			t.Errorf("FromStatus(%q): got %q", tc.status, s.Name())
		}
	}
}

func TestFromStatus_Unknown(t *testing.T) {
	_, err := FromStatus("BOGUS")
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}
