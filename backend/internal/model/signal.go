package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ComponentType string

const (
	ComponentAPI     ComponentType = "API"
	ComponentMCPHost ComponentType = "MCP_HOST"
	ComponentCache   ComponentType = "CACHE"
	ComponentQueue   ComponentType = "QUEUE"
	ComponentRDBMS   ComponentType = "RDBMS"
	ComponentNoSQL   ComponentType = "NOSQL"
	ComponentOther   ComponentType = "OTHER"
)

func (c ComponentType) Valid() bool {
	switch c {
	case ComponentAPI, ComponentMCPHost, ComponentCache, ComponentQueue,
		ComponentRDBMS, ComponentNoSQL, ComponentOther:
		return true
	}
	return false
}

type Severity string

const (
	SeverityP0 Severity = "P0"
	SeverityP1 Severity = "P1"
	SeverityP2 Severity = "P2"
	SeverityP3 Severity = "P3"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityP0, SeverityP1, SeverityP2, SeverityP3:
		return true
	}
	return false
}

type Signal struct {
	SignalID      uuid.UUID       `json:"signal_id"`
	ComponentID   string          `json:"component_id"`
	ComponentType ComponentType   `json:"component_type"`
	Severity      Severity        `json:"severity"`
	Timestamp     time.Time       `json:"timestamp"`
	Source        string          `json:"source"`
	Payload       json.RawMessage `json:"payload"`
}

var (
	ErrMissingComponentID   = errors.New("component_id is required")
	ErrInvalidComponentType = errors.New("component_type must be one of API|MCP_HOST|CACHE|QUEUE|RDBMS|NOSQL|OTHER")
	ErrInvalidSeverity      = errors.New("severity must be one of P0|P1|P2|P3")
	ErrMissingSource        = errors.New("source is required")
)

func (s *Signal) Validate() error {
	if s.ComponentID == "" {
		return ErrMissingComponentID
	}
	if !s.ComponentType.Valid() {
		return fmt.Errorf("%w: got %q", ErrInvalidComponentType, s.ComponentType)
	}
	if !s.Severity.Valid() {
		return fmt.Errorf("%w: got %q", ErrInvalidSeverity, s.Severity)
	}
	if s.Source == "" {
		return ErrMissingSource
	}
	return nil
}

func (s *Signal) ApplyDefaults(now time.Time) {
	if s.SignalID == uuid.Nil {
		s.SignalID = uuid.New()
	}
	if s.Timestamp.IsZero() {
		s.Timestamp = now
	}
}
