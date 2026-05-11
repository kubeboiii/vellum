package model

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusOpen          Status = "OPEN"
	StatusInvestigating Status = "INVESTIGATING"
	StatusResolved      Status = "RESOLVED"
	StatusClosed        Status = "CLOSED"
)

type WorkItem struct {
	ID            uuid.UUID     `json:"id"`
	ComponentID   string        `json:"component_id"`
	ComponentType ComponentType `json:"component_type"`
	Severity      Severity      `json:"severity"`
	Status        Status        `json:"status"`
	SignalCount   int           `json:"signal_count"`
	FirstSignalTS time.Time     `json:"first_signal_ts"`
	LastSignalTS  time.Time     `json:"last_signal_ts"`

	MTTRSeconds   *int       `json:"mttr_seconds,omitempty"`
	IncidentStart *time.Time `json:"incident_start,omitempty"`
	IncidentEnd   *time.Time `json:"incident_end,omitempty"`
	ClosedAt      *time.Time `json:"closed_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func NewWorkItem(id uuid.UUID, sig Signal) WorkItem {
	now := time.Now().UTC()
	return WorkItem{
		ID:            id,
		ComponentID:   sig.ComponentID,
		ComponentType: sig.ComponentType,
		Severity:      sig.Severity,
		Status:        StatusOpen,
		SignalCount:   1,
		FirstSignalTS: sig.Timestamp,
		LastSignalTS:  sig.Timestamp,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}
