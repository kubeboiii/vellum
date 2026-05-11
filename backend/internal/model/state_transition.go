package model

import (
	"time"

	"github.com/google/uuid"
)

type StateTransition struct {
	ID         uuid.UUID `json:"id"`
	WorkItemID uuid.UUID `json:"work_item_id"`
	FromState  Status    `json:"from_state"`
	ToState    Status    `json:"to_state"`
	Reason     string    `json:"reason,omitempty"`
	Actor      string    `json:"actor,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
