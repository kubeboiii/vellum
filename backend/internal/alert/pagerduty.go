package alert

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/kubeboiii/vellum/internal/model"
)

type PagerDutyStub struct{}

func (PagerDutyStub) Name() string { return "pagerduty_stub" }

type pdEvent struct {
	IncidentID  string `json:"incident_id"`
	Severity    string `json:"severity"`
	Source      string `json:"source"`
	Summary     string `json:"summary"`
	SignalCount int    `json:"signal_count"`
	Timestamp   string `json:"timestamp"`
}

func (PagerDutyStub) Dispatch(_ context.Context, wi model.WorkItem) error {
	ev := pdEvent{
		IncidentID:  wi.ID.String(),
		Severity:    string(wi.Severity),
		Source:      wi.ComponentID,
		Summary:     "Vellum incident on " + wi.ComponentID,
		SignalCount: wi.SignalCount,
		Timestamp:   wi.LastSignalTS.UTC().Format(time.RFC3339),
	}

	body, _ := json.Marshal(ev)
	log.Printf("[alert pagerduty_stub] %s", body)
	return nil
}
