package model

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validSignal() Signal {
	return Signal{
		ComponentID:   "RDBMS_PRIMARY_01",
		ComponentType: ComponentRDBMS,
		Severity:      SeverityP0,
		Source:        "datadog",
		Payload:       json.RawMessage(`{"msg":"hi"}`),
	}
}

func TestValidate_OK(t *testing.T) {
	s := validSignal()
	if err := s.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidate_MissingComponentID(t *testing.T) {
	s := validSignal()
	s.ComponentID = ""
	if err := s.Validate(); !errors.Is(err, ErrMissingComponentID) {
		t.Fatalf("expected ErrMissingComponentID, got %v", err)
	}
}

func TestValidate_BadComponentType(t *testing.T) {
	s := validSignal()
	s.ComponentType = "WAT"
	if err := s.Validate(); !errors.Is(err, ErrInvalidComponentType) {
		t.Fatalf("expected ErrInvalidComponentType, got %v", err)
	}
}

func TestValidate_BadSeverity(t *testing.T) {
	s := validSignal()
	s.Severity = "P9"
	if err := s.Validate(); !errors.Is(err, ErrInvalidSeverity) {
		t.Fatalf("expected ErrInvalidSeverity, got %v", err)
	}
}

func TestValidate_MissingSource(t *testing.T) {
	s := validSignal()
	s.Source = ""
	if err := s.Validate(); !errors.Is(err, ErrMissingSource) {
		t.Fatalf("expected ErrMissingSource, got %v", err)
	}
}

func TestApplyDefaults_FillsMissing(t *testing.T) {
	s := validSignal()
	now := time.Date(2026, 5, 12, 3, 14, 15, 0, time.UTC)
	s.ApplyDefaults(now)
	if s.SignalID == uuid.Nil {
		t.Fatal("SignalID should be generated")
	}
	if !s.Timestamp.Equal(now) {
		t.Fatalf("timestamp should be %v, got %v", now, s.Timestamp)
	}
}

func TestApplyDefaults_PreservesGiven(t *testing.T) {
	id := uuid.New()
	ts := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	s := validSignal()
	s.SignalID = id
	s.Timestamp = ts
	s.ApplyDefaults(time.Now())
	if s.SignalID != id {
		t.Fatal("SignalID was overwritten")
	}
	if !s.Timestamp.Equal(ts) {
		t.Fatal("Timestamp was overwritten")
	}
}

func TestJSONRoundTrip(t *testing.T) {
	in := validSignal()
	in.ApplyDefaults(time.Now())
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Signal
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SignalID != in.SignalID || out.ComponentID != in.ComponentID {
		t.Fatalf("round-trip lost fields: %+v vs %+v", in, out)
	}
}
