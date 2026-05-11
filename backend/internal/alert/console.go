package alert

import (
	"context"
	"log"

	"github.com/kubeboiii/ims/internal/model"
)

// ConsoleAlerter is the always-present fallback. Logs a one-line
// summary to stdout in a parseable key=value format. Used for P3
// signals (FR-6.2) and as the final-fallback if a matched alerter is
// missing from the registry (e.g., misconfigured Slack URL).
type ConsoleAlerter struct{}

// Name identifies the alerter in logs and the Registry's name map.
func (ConsoleAlerter) Name() string { return "console" }

// Dispatch emits one log line and returns nil. Cannot fail; there's
// no I/O to lose.
func (ConsoleAlerter) Dispatch(_ context.Context, wi model.WorkItem) error {
	log.Printf(
		"[alert console] work_item_id=%s severity=%s component=%s status=%s signal_count=%d",
		wi.ID, wi.Severity, wi.ComponentID, wi.Status, wi.SignalCount,
	)
	return nil
}
