package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RootCauseCategory is the enum from 00-master-prd §4.5. Used both as
// a column value (matched by the rca_category_chk constraint in
// migration 004) and as the form picker in the RCA UI (Phase 5).
type RootCauseCategory string

const (
	CategoryCodeDefect         RootCauseCategory = "CODE_DEFECT"
	CategoryInfrastructure     RootCauseCategory = "INFRASTRUCTURE"
	CategoryConfigChange       RootCauseCategory = "CONFIG_CHANGE"
	CategoryExternalDependency RootCauseCategory = "EXTERNAL_DEPENDENCY"
	CategoryCapacity           RootCauseCategory = "CAPACITY"
	CategoryHumanError         RootCauseCategory = "HUMAN_ERROR"
	CategoryOther              RootCauseCategory = "OTHER"
)

// Valid returns true when the value is one of the documented enum members.
func (c RootCauseCategory) Valid() bool {
	switch c {
	case CategoryCodeDefect, CategoryInfrastructure, CategoryConfigChange,
		CategoryExternalDependency, CategoryCapacity, CategoryHumanError, CategoryOther:
		return true
	}
	return false
}

// rcaMinTextLength is the minimum allowed length for the free-text RCA
// fields (00-master-prd §4.5: "min 20 chars"). Both `fix_applied` and
// `prevention_steps` must clear this bar. We mirror the DB CHECK
// constraints in migration 004 here so the API returns a clean 422
// instead of letting Postgres reject with a constraint-violation
// error.
const rcaMinTextLength = 20

// RCA is the post-mortem record attached to a closed Work Item.
// Exactly one RCA per Work Item (UNIQUE on work_item_id in Postgres).
//
// Note on the field order: matches the JSON schema we'll expose in
// Phase 5 (`POST /v1/incidents/:id/rca`). Keep the json tags stable
// — the dashboard form will rely on these.
type RCA struct {
	ID                uuid.UUID         `json:"id"`
	WorkItemID        uuid.UUID         `json:"work_item_id"`
	IncidentStart     time.Time         `json:"incident_start"`
	IncidentEnd       time.Time         `json:"incident_end"`
	RootCauseCategory RootCauseCategory `json:"root_cause_category"`
	FixApplied        string            `json:"fix_applied"`
	PreventionSteps   string            `json:"prevention_steps"`
	SubmittedBy       string            `json:"submitted_by"`
	CreatedAt         time.Time         `json:"created_at"`
}

// Sentinel errors. Exported so handlers can map to 422 responses with
// the right field name.
var (
	ErrMissingRCA            = errors.New("rca is required to close a work item")
	ErrInvalidCategory       = errors.New("root_cause_category must be one of CODE_DEFECT|INFRASTRUCTURE|CONFIG_CHANGE|EXTERNAL_DEPENDENCY|CAPACITY|HUMAN_ERROR|OTHER")
	ErrFixAppliedTooShort    = fmt.Errorf("fix_applied must be at least %d characters", rcaMinTextLength)
	ErrPreventionTooShort    = fmt.Errorf("prevention_steps must be at least %d characters", rcaMinTextLength)
	ErrIncidentTimesReversed = errors.New("incident_end must be at or after incident_start")
	ErrSubmittedByMissing    = errors.New("submitted_by is required")
)

// FieldError pairs an offending field name with a sentinel error. The
// handler can collect a `[]FieldError` and shape a 422 response body
// that points the user at every field they need to fix in one round-trip.
type FieldError struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

// Validate enforces every RCA rule in 00-master-prd §4.5. Returns the
// flat list of field errors so the handler can surface them all at
// once. Returns nil on success.
//
// The DB has the same constraints as a safety net (migration 004), but
// app-level validation produces nicer error messages and runs before
// the SQL round-trip.
func (r *RCA) Validate() []FieldError {
	var errs []FieldError

	if !r.RootCauseCategory.Valid() {
		errs = append(errs, FieldError{
			Field: "root_cause_category",
			Error: ErrInvalidCategory.Error(),
		})
	}
	if len(strings.TrimSpace(r.FixApplied)) < rcaMinTextLength {
		errs = append(errs, FieldError{
			Field: "fix_applied",
			Error: ErrFixAppliedTooShort.Error(),
		})
	}
	if len(strings.TrimSpace(r.PreventionSteps)) < rcaMinTextLength {
		errs = append(errs, FieldError{
			Field: "prevention_steps",
			Error: ErrPreventionTooShort.Error(),
		})
	}
	if !r.IncidentEnd.IsZero() && r.IncidentEnd.Before(r.IncidentStart) {
		errs = append(errs, FieldError{
			Field: "incident_end",
			Error: ErrIncidentTimesReversed.Error(),
		})
	}
	if strings.TrimSpace(r.SubmittedBy) == "" {
		errs = append(errs, FieldError{
			Field: "submitted_by",
			Error: ErrSubmittedByMissing.Error(),
		})
	}
	return errs
}

// ApplyDefaults fills in server-side fields the caller may omit.
//
// Per 00-master-prd §4.5: incident_start defaults to the first signal
// timestamp (passed in by the workflow engine from the WI row);
// incident_end defaults to submission time (now); submitted_by
// defaults to "system" when auth is absent (PRD NG1).
//
// `now` is injectable so tests can freeze time.
func (r *RCA) ApplyDefaults(workItemFirstSignalTS time.Time, now time.Time) {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.IncidentStart.IsZero() {
		r.IncidentStart = workItemFirstSignalTS
	}
	if r.IncidentEnd.IsZero() {
		r.IncidentEnd = now
	}
	if strings.TrimSpace(r.SubmittedBy) == "" {
		r.SubmittedBy = "system"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
}

// MTTRSeconds returns the duration between the incident's start and
// end (00-master-prd §4.5). Computed here so the workflow engine can
// stamp it on the Work Item when entering ClosedState — that keeps
// MTTR computation in one place. Returns 0 if either timestamp is zero.
func (r *RCA) MTTRSeconds() int {
	if r.IncidentStart.IsZero() || r.IncidentEnd.IsZero() {
		return 0
	}
	d := r.IncidentEnd.Sub(r.IncidentStart)
	if d < 0 {
		return 0
	}
	return int(d.Seconds())
}
