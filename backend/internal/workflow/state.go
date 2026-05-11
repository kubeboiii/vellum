package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/kubeboiii/vellum/internal/model"
)

type State interface {
	Name() model.Status
	CanTransitionTo(next State, tctx TransitionContext) error
	OnEnter(ctx context.Context, wi *model.WorkItem, tctx TransitionContext) error
}

type TransitionContext struct {
	RCA    *model.RCA
	Actor  string
	Reason string
}

var (
	ErrInvalidTransition = errors.New("workflow: transition not allowed from current state")
	ErrIncompleteRCA     = errors.New("workflow: RCA is incomplete; see fields")
)

var ErrMissingRCA = model.ErrMissingRCA

type OpenState struct{}

func (OpenState) Name() model.Status { return model.StatusOpen }

func (OpenState) CanTransitionTo(next State, _ TransitionContext) error {
	if _, ok := next.(InvestigatingState); ok {
		return nil
	}
	return fmt.Errorf("%w: OPEN -> %s", ErrInvalidTransition, next.Name())
}

func (OpenState) OnEnter(_ context.Context, _ *model.WorkItem, _ TransitionContext) error {
	return nil
}

type InvestigatingState struct{}

func (InvestigatingState) Name() model.Status { return model.StatusInvestigating }

func (InvestigatingState) CanTransitionTo(next State, _ TransitionContext) error {
	if _, ok := next.(ResolvedState); ok {
		return nil
	}
	return fmt.Errorf("%w: INVESTIGATING -> %s", ErrInvalidTransition, next.Name())
}

func (InvestigatingState) OnEnter(_ context.Context, _ *model.WorkItem, _ TransitionContext) error {
	return nil
}

type ResolvedState struct{}

func (ResolvedState) Name() model.Status { return model.StatusResolved }

func (ResolvedState) CanTransitionTo(next State, tctx TransitionContext) error {
	if _, ok := next.(ClosedState); !ok {
		return fmt.Errorf("%w: RESOLVED -> %s", ErrInvalidTransition, next.Name())
	}
	if tctx.RCA == nil {
		return ErrMissingRCA
	}
	if errs := tctx.RCA.Validate(); len(errs) > 0 {

		return &IncompleteRCAError{Fields: errs}
	}
	return nil
}

func (ResolvedState) OnEnter(_ context.Context, _ *model.WorkItem, _ TransitionContext) error {
	return nil
}

type ClosedState struct{}

func (ClosedState) Name() model.Status { return model.StatusClosed }

func (ClosedState) CanTransitionTo(next State, _ TransitionContext) error {
	return fmt.Errorf("%w: CLOSED is terminal -> %s", ErrInvalidTransition, next.Name())
}

func (ClosedState) OnEnter(_ context.Context, wi *model.WorkItem, tctx TransitionContext) error {
	if tctx.RCA == nil {

		return fmt.Errorf("workflow: ClosedState.OnEnter called without RCA (engine bug)")
	}
	mttr := tctx.RCA.MTTRSeconds()
	wi.MTTRSeconds = &mttr
	closedAt := tctx.RCA.IncidentEnd
	wi.ClosedAt = &closedAt
	wi.IncidentStart = &tctx.RCA.IncidentStart
	wi.IncidentEnd = &tctx.RCA.IncidentEnd
	return nil
}

type IncompleteRCAError struct {
	Fields []model.FieldError
}

func (e *IncompleteRCAError) Error() string {
	return ErrIncompleteRCA.Error()
}

func (e *IncompleteRCAError) Unwrap() error { return ErrIncompleteRCA }

func FromStatus(s model.Status) (State, error) {
	switch s {
	case model.StatusOpen:
		return OpenState{}, nil
	case model.StatusInvestigating:
		return InvestigatingState{}, nil
	case model.StatusResolved:
		return ResolvedState{}, nil
	case model.StatusClosed:
		return ClosedState{}, nil
	}
	return nil, fmt.Errorf("workflow: unknown status %q", s)
}
