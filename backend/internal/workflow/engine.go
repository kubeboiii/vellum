package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/vellum/internal/model"
)

type TxRunner interface {
	BeginTx(ctx context.Context) (Tx, error)
}

type Tx interface {
	LockWorkItem(ctx context.Context, id uuid.UUID) (model.WorkItem, error)
	UpdateWorkItemStateAndMTTR(ctx context.Context, wi model.WorkItem) error
	InsertStateTransition(ctx context.Context, t model.StateTransition) error
	InsertRCA(ctx context.Context, rca model.RCA) error
	Commit() error
	Rollback() error
}

var ErrWorkItemNotFound = errors.New("workflow: work item not found")

type Engine struct {
	tx TxRunner
}

func NewEngine(tx TxRunner) *Engine {
	return &Engine{tx: tx}
}

func (e *Engine) Transition(ctx context.Context, id uuid.UUID, next State, tctx TransitionContext) (model.WorkItem, error) {
	tx, err := e.tx.BeginTx(ctx)
	if err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: begin tx: %w", err)
	}

	defer func() { _ = tx.Rollback() }()

	wi, err := tx.LockWorkItem(ctx, id)
	if err != nil {
		return model.WorkItem{}, err
	}

	current, err := FromStatus(wi.Status)
	if err != nil {
		return model.WorkItem{}, err
	}
	if err := current.CanTransitionTo(next, tctx); err != nil {
		return model.WorkItem{}, err
	}

	if err := next.OnEnter(ctx, &wi, tctx); err != nil {
		return model.WorkItem{}, err
	}
	wi.Status = next.Name()

	if err := tx.UpdateWorkItemStateAndMTTR(ctx, wi); err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: update work_item: %w", err)
	}
	transition := model.StateTransition{
		ID:         uuid.New(),
		WorkItemID: wi.ID,
		FromState:  current.Name(),
		ToState:    next.Name(),
		Reason:     tctx.Reason,
		Actor:      tctx.Actor,
	}
	if err := tx.InsertStateTransition(ctx, transition); err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: insert transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.WorkItem{}, fmt.Errorf("workflow: commit: %w", err)
	}
	return wi, nil
}

func (e *Engine) CloseWithRCA(ctx context.Context, workItemID uuid.UUID, rca *model.RCA, actor string) (model.WorkItem, model.RCA, error) {
	if rca == nil {
		return model.WorkItem{}, model.RCA{}, ErrMissingRCA
	}

	tx, err := e.tx.BeginTx(ctx)
	if err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	wi, err := tx.LockWorkItem(ctx, workItemID)
	if err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}

	rca.WorkItemID = wi.ID
	rca.ApplyDefaults(wi.FirstSignalTS, time.Now().UTC())

	current, err := FromStatus(wi.Status)
	if err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}
	tctx := TransitionContext{RCA: rca, Actor: actor}
	if err := current.CanTransitionTo(ClosedState{}, tctx); err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}

	if err := (ClosedState{}).OnEnter(ctx, &wi, tctx); err != nil {
		return model.WorkItem{}, model.RCA{}, err
	}
	wi.Status = model.StatusClosed

	if err := tx.InsertRCA(ctx, *rca); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: insert rca: %w", err)
	}
	if err := tx.UpdateWorkItemStateAndMTTR(ctx, wi); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: update work_item: %w", err)
	}
	transition := model.StateTransition{
		ID:         uuid.New(),
		WorkItemID: wi.ID,
		FromState:  current.Name(),
		ToState:    model.StatusClosed,
		Reason:     "RCA submitted",
		Actor:      actor,
	}
	if err := tx.InsertStateTransition(ctx, transition); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: insert transition: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return model.WorkItem{}, model.RCA{}, fmt.Errorf("workflow: commit: %w", err)
	}
	return wi, *rca, nil
}
