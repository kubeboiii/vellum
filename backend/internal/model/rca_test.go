package model

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validRCA() RCA {
	return RCA{
		WorkItemID:        uuid.New(),
		IncidentStart:     time.Date(2026, 5, 12, 3, 0, 0, 0, time.UTC),
		IncidentEnd:       time.Date(2026, 5, 12, 3, 30, 0, 0, time.UTC),
		RootCauseCategory: CategoryInfrastructure,
		FixApplied:        "Rebooted the cache cluster and increased the connection pool.",
		PreventionSteps:   "Add a synthetic monitor for connection-pool saturation.",
		SubmittedBy:       "sre@example.com",
	}
}

func TestRCAValidate_OK(t *testing.T) {
	rca := validRCA()
	if errs := rca.Validate(); errs != nil {
		t.Fatalf("expected nil errors, got %v", errs)
	}
}

func TestValidate_BadCategory(t *testing.T) {
	rca := validRCA()
	rca.RootCauseCategory = "WAT"
	errs := rca.Validate()
	if len(errs) != 1 || errs[0].Field != "root_cause_category" {
		t.Fatalf("expected one error on root_cause_category, got %v", errs)
	}
}

func TestValidate_FixTooShort(t *testing.T) {
	rca := validRCA()
	rca.FixApplied = "too short"
	errs := rca.Validate()
	if len(errs) != 1 || errs[0].Field != "fix_applied" {
		t.Fatalf("expected one error on fix_applied, got %v", errs)
	}
}

func TestValidate_PreventionTooShort(t *testing.T) {
	rca := validRCA()
	rca.PreventionSteps = strings.Repeat("a", rcaMinTextLength-1)
	errs := rca.Validate()
	if len(errs) != 1 || errs[0].Field != "prevention_steps" {
		t.Fatalf("expected one error on prevention_steps, got %v", errs)
	}
}

// TestValidate_ReversedTimes: incident_end < incident_start → error.
func TestValidate_ReversedTimes(t *testing.T) {
	rca := validRCA()
	rca.IncidentStart, rca.IncidentEnd = rca.IncidentEnd, rca.IncidentStart
	errs := rca.Validate()
	if len(errs) != 1 || errs[0].Field != "incident_end" {
		t.Fatalf("expected one error on incident_end, got %v", errs)
	}
}

// TestValidate_MultipleErrors: surface them all at once so the user can
// fix them in one round-trip.
func TestValidate_MultipleErrors(t *testing.T) {
	rca := validRCA()
	rca.RootCauseCategory = "BAD"
	rca.FixApplied = ""
	rca.SubmittedBy = ""
	errs := rca.Validate()
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
}

// TestValidate_WhitespaceOnlyFields: TrimSpace should treat
// whitespace-only fix_applied and prevention_steps as empty.
func TestValidate_WhitespaceOnlyFields(t *testing.T) {
	rca := validRCA()
	rca.FixApplied = "                                           " // 43 spaces
	errs := rca.Validate()
	found := false
	for _, e := range errs {
		if e.Field == "fix_applied" {
			found = true
		}
	}
	if !found {
		t.Errorf("whitespace-only fix_applied should fail validation: %v", errs)
	}
}

func TestRCAApplyDefaults_FillsMissing(t *testing.T) {
	first := time.Date(2026, 5, 12, 2, 0, 0, 0, time.UTC)
	now := time.Date(2026, 5, 12, 4, 0, 0, 0, time.UTC)
	rca := RCA{
		RootCauseCategory: CategoryOther,
		FixApplied:        strings.Repeat("x", 21),
		PreventionSteps:   strings.Repeat("y", 21),
	}
	rca.ApplyDefaults(first, now)
	if rca.ID == uuid.Nil {
		t.Error("ID should be generated")
	}
	if !rca.IncidentStart.Equal(first) {
		t.Errorf("IncidentStart: want %v, got %v", first, rca.IncidentStart)
	}
	if !rca.IncidentEnd.Equal(now) {
		t.Errorf("IncidentEnd: want %v, got %v", now, rca.IncidentEnd)
	}
	if rca.SubmittedBy != "system" {
		t.Errorf("SubmittedBy: want 'system', got %q", rca.SubmittedBy)
	}
}

func TestRCAApplyDefaults_PreservesGiven(t *testing.T) {
	id := uuid.New()
	rca := RCA{
		ID:            id,
		IncidentStart: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		IncidentEnd:   time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		SubmittedBy:   "alice@example.com",
	}
	rca.ApplyDefaults(time.Now(), time.Now())
	if rca.ID != id {
		t.Error("ID was overwritten")
	}
	if rca.SubmittedBy != "alice@example.com" {
		t.Errorf("SubmittedBy was overwritten: %q", rca.SubmittedBy)
	}
}

func TestMTTRSeconds(t *testing.T) {
	rca := validRCA() // 30 minutes apart
	if got := rca.MTTRSeconds(); got != 1800 {
		t.Errorf("want 1800s (30min), got %d", got)
	}

	// Zero values → 0, not negative.
	empty := RCA{}
	if got := empty.MTTRSeconds(); got != 0 {
		t.Errorf("empty MTTR want 0, got %d", got)
	}
}
