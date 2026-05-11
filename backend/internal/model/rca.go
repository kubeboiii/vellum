package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

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

func (c RootCauseCategory) Valid() bool {
	switch c {
	case CategoryCodeDefect, CategoryInfrastructure, CategoryConfigChange,
		CategoryExternalDependency, CategoryCapacity, CategoryHumanError, CategoryOther:
		return true
	}
	return false
}

const rcaMinTextLength = 20

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

var (
	ErrMissingRCA            = errors.New("rca is required to close a work item")
	ErrInvalidCategory       = errors.New("root_cause_category must be one of CODE_DEFECT|INFRASTRUCTURE|CONFIG_CHANGE|EXTERNAL_DEPENDENCY|CAPACITY|HUMAN_ERROR|OTHER")
	ErrFixAppliedTooShort    = fmt.Errorf("fix_applied must be at least %d characters", rcaMinTextLength)
	ErrPreventionTooShort    = fmt.Errorf("prevention_steps must be at least %d characters", rcaMinTextLength)
	ErrIncidentTimesReversed = errors.New("incident_end must be at or after incident_start")
	ErrSubmittedByMissing    = errors.New("submitted_by is required")
)

type FieldError struct {
	Field string `json:"field"`
	Error string `json:"error"`
}

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
