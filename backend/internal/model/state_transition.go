package model

import (
	"time"

	"github.com/google/uuid"
)

// StateTransition is one row in the `state_transitions` audit table
// (migration 002). The workflow engine writes one per successful
// transition, atomically with the work_items UPDATE (FR-4.4).
type StateTransition struct {
	ID         uuid.UUID `json:"id"`
	WorkItemID uuid.UUID `json:"work_item_id"`
	FromState  Status    `json:"from_state"`
	ToState    Status    `json:"to_state"`
	Reason     string    `json:"reason,omitempty"`
	Actor      string    `json:"actor,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
