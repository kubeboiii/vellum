// Package alert implements the Strategy pattern for incident
// notifications (00-master-prd §4.6, 01-architecture §7.1). Each
// strategy is a tiny struct satisfying the Alerter interface; the
// Registry picks one per Work Item based on severity.
//
// Why Strategy and not a giant switch:
//   - Adding a new channel (Microsoft Teams, OpsGenie) means writing
//     one struct and adding one Rule. Zero changes to processor,
//     workflow, or persistence.
//   - State-specific behaviour (Slack does HTTP, PagerDuty logs JSON,
//     Console logs a one-liner) lives in the concrete type, not in
//     conditionals on severity.
//   - Tests can swap a fake alerter with one line of code.
//
// FR-6.4: alert dispatch is async and isolated. A failing alerter
// cannot block ingestion or workflow. The processor calls
// `go alerter.Dispatch(...)` after the Postgres write succeeds, with
// its own 5-second timeout context.
package alert

import (
	"context"

	"github.com/kubeboiii/ims/internal/model"
)

// Alerter is the contract every notification channel implements. Tiny
// on purpose — fewer methods means fewer mocks in tests.
type Alerter interface {
	// Name identifies the alerter in logs ("pagerduty_stub",
	// "slack_webhook", "console"). Stable across phases.
	Name() string
	// Dispatch fires the notification for `wi`. It MUST honour
	// `ctx`'s deadline so a slow webhook doesn't pile up goroutines.
	// Errors are logged by the caller; we don't return them up the
	// stack because FR-6.4 says alerts can't block workflow.
	Dispatch(ctx context.Context, wi model.WorkItem) error
}

// Rule is one entry in the Registry's match table. The first rule
// whose Matches returns true claims the Work Item.
//
// We use a closure rather than fields like "MinSeverity" because the
// rules are tiny and Go closures are cheap; the closure form keeps
// rule logic next to the rule wiring (one-line per registration).
type Rule struct {
	Name        string
	Matches     func(model.WorkItem) bool
	AlerterName string
}

// Registry holds the ordered match rules plus a name → Alerter map.
// ForWorkItem walks the rules and returns the first match's Alerter,
// or the "console" fallback if nothing matches.
type Registry struct {
	rules    []Rule
	alerters map[string]Alerter
}

// NewRegistry wires the alerters with their match rules. Order
// matters — PagerDuty for P0 must come before Slack for P1/P2.
// The "console" fallback is always registered last.
func NewRegistry(alerters map[string]Alerter, rules []Rule) *Registry {
	// Defensive copy so callers can't mutate our state after construction.
	cp := make(map[string]Alerter, len(alerters))
	for k, v := range alerters {
		cp[k] = v
	}
	return &Registry{rules: rules, alerters: cp}
}

// ForWorkItem returns the Alerter that should be invoked for this WI.
// Falls back to the `console` alerter if no rule matches and `console`
// is registered. Returns nil if no fallback either — callers can
// treat nil as "no-op".
func (r *Registry) ForWorkItem(wi model.WorkItem) Alerter {
	for _, rule := range r.rules {
		if rule.Matches(wi) {
			if a, ok := r.alerters[rule.AlerterName]; ok {
				return a
			}
		}
	}
	return r.alerters["console"]
}

// ---- Standard rule helpers ----

// SeverityRule returns a Rule that matches Work Items at any of the
// given severities. Handy for the v1 alerters (FR-6.2):
//   - PagerDutyStub: P0
//   - SlackWebhook:  P1, P2
//   - Console:       P3 (and fallback)
func SeverityRule(name, alerterName string, sevs ...model.Severity) Rule {
	set := make(map[model.Severity]struct{}, len(sevs))
	for _, s := range sevs {
		set[s] = struct{}{}
	}
	return Rule{
		Name:        name,
		AlerterName: alerterName,
		Matches: func(wi model.WorkItem) bool {
			_, ok := set[wi.Severity]
			return ok
		},
	}
}
