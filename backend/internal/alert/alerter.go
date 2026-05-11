package alert

import (
	"context"

	"github.com/kubeboiii/vellum/internal/model"
)

type Alerter interface {

	Name() string

	Dispatch(ctx context.Context, wi model.WorkItem) error
}

type Rule struct {
	Name        string
	Matches     func(model.WorkItem) bool
	AlerterName string
}

type Registry struct {
	rules    []Rule
	alerters map[string]Alerter
}

func NewRegistry(alerters map[string]Alerter, rules []Rule) *Registry {

	cp := make(map[string]Alerter, len(alerters))
	for k, v := range alerters {
		cp[k] = v
	}
	return &Registry{rules: rules, alerters: cp}
}

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
