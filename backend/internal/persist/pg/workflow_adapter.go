package pg

import (
	"context"

	"github.com/kubeboiii/vellum/internal/workflow"
)

type WorkflowTxRunner struct {
	repo *WorkItemRepository
}

func NewWorkflowTxRunner(repo *WorkItemRepository) *WorkflowTxRunner {
	return &WorkflowTxRunner{repo: repo}
}

func (w *WorkflowTxRunner) BeginTx(ctx context.Context) (workflow.Tx, error) {
	return w.repo.BeginTx(ctx)
}
