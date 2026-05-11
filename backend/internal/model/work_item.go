package model

import (
	"time"

	"github.com/google/uuid"
)

// Status is the Work Item lifecycle enum from 00-master-prd §4.4.1.
// Phase 3 only creates rows in OPEN state; Phase 4's State pattern
// drives the transitions OPEN → INVESTIGATING → RESOLVED → CLOSED.
type Status string

const (
	StatusOpen          Status = "OPEN"
	StatusInvestigating Status = "INVESTIGATING"
	StatusResolved      Status = "RESOLVED"
	StatusClosed        Status = "CLOSED"
)

// WorkItem is one aggregated incident on one component. Created by the
// debouncer when a quiet component first emits a signal; subsequent
// signals within the window bump SignalCount + LastSignalTS rather than
// creating a new row. Source of truth: Postgres.
//
// Nullable fields (MTTRSeconds, IncidentStart, IncidentEnd, ClosedAt)
// are pointers so we can distinguish "not set yet" from zero values.
// They get populated in Phase 4 when the State pattern transitions us
// through INVESTIGATING → RESOLVED → CLOSED.
type WorkItem struct {
	ID            uuid.UUID
	ComponentID   string
	ComponentType ComponentType
	Severity      Severity
	Status        Status
	SignalCount   int
	FirstSignalTS time.Time
	LastSignalTS  time.Time

	MTTRSeconds   *int
	IncidentStart *time.Time
	IncidentEnd   *time.Time
	ClosedAt      *time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewWorkItem builds an OPEN-state Work Item from the first signal of a
// debounce window. Called by the processor when the Lua script returns
// action=CREATED. ID is passed in (it's the same UUID we sent to the
// Lua script as the candidate work_item_id).
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
