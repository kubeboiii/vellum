// Package model defines the cross-package types that flow through the IMS:
// Signal (this file), WorkItem, RCA, State (Phase 3+).
//
// Keep this package pure: no I/O, no logging, no dependencies on internal/
// packages. Everything here is a value type plus its validator.
package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ComponentType is the enum from 00-master-prd §4.2.
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

// Severity is the enum from 00-master-prd §4.2.
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

// Signal is the wire-format payload accepted by POST /v1/signals and (Phase
// 5) the gRPC stream. Field tags match the schema in 00-master-prd §4.2.
//
// `Payload` stays as raw JSON so we (a) avoid unmarshalling free-form data
// twice on the hot path and (b) can stream it straight into Mongo in Phase
// 3 without reflecting over it.
type Signal struct {
	SignalID      uuid.UUID       `json:"signal_id"`
	ComponentID   string          `json:"component_id"`
	ComponentType ComponentType   `json:"component_type"`
	Severity      Severity        `json:"severity"`
	Timestamp     time.Time       `json:"timestamp"`
	Source        string          `json:"source"`
	Payload       json.RawMessage `json:"payload"`
}

// Validation errors. Exported so handlers can map them to 400 responses
// without string-matching.
var (
	ErrMissingComponentID   = errors.New("component_id is required")
	ErrInvalidComponentType = errors.New("component_type must be one of API|MCP_HOST|CACHE|QUEUE|RDBMS|NOSQL|OTHER")
	ErrInvalidSeverity      = errors.New("severity must be one of P0|P1|P2|P3")
	ErrMissingSource        = errors.New("source is required")
)

// Validate enforces the FR-2 schema. The handler is expected to call this
// after JSON unmarshal and before enqueue. Server-side defaults (signal_id,
// timestamp) are *not* applied here — the caller fills them in via
// ApplyDefaults so that Validate stays purely a pure check.
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

// ApplyDefaults fills in server-side fields when the caller omits them.
// Generating a UUID per signal costs ~50ns, well under the 50ms p99 budget
// in NFR-1.1, so we do it on the hot path rather than deferring to a worker.
func (s *Signal) ApplyDefaults(now time.Time) {
	if s.SignalID == uuid.Nil {
		s.SignalID = uuid.New()
	}
	if s.Timestamp.IsZero() {
		s.Timestamp = now
	}
}
