package pg

import (
	"context"

	"github.com/kubeboiii/ims/internal/workflow"
)

// WorkflowTxRunner adapts *WorkItemRepository to workflow.TxRunner.
// We can't have *WorkItemRepository directly implement TxRunner
// because Go's structural typing requires the return type of BeginTx
// to be exactly `workflow.Tx` — and `*workItemTx` (the concrete
// type returned by the underlying BeginTx) only happens to implement
// that interface. Wrapping here makes the conversion explicit.
//
// main.go does:
//
//	engine := workflow.NewEngine(pg.NewWorkflowTxRunner(workItems))
type WorkflowTxRunner struct {
	repo *WorkItemRepository
}

// NewWorkflowTxRunner wraps a WorkItemRepository so it satisfies
// workflow.TxRunner.
func NewWorkflowTxRunner(repo *WorkItemRepository) *WorkflowTxRunner {
	return &WorkflowTxRunner{repo: repo}
}

// BeginTx delegates to the repo, wrapping the concrete *workItemTx
// in the workflow.Tx interface.
func (w *WorkflowTxRunner) BeginTx(ctx context.Context) (workflow.Tx, error) {
	return w.repo.BeginTx(ctx)
}
