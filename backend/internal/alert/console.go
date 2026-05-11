package alert

import (
	"context"
	"log"

	"github.com/kubeboiii/vellum/internal/model"
)

type ConsoleAlerter struct{}

func (ConsoleAlerter) Name() string { return "console" }

func (ConsoleAlerter) Dispatch(_ context.Context, wi model.WorkItem) error {
	log.Printf(
		"[alert console] work_item_id=%s severity=%s component=%s status=%s signal_count=%d",
		wi.ID, wi.Severity, wi.ComponentID, wi.Status, wi.SignalCount,
	)
	return nil
}
