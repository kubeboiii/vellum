package alert

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubeboiii/ims/internal/model"
)

// recordingAlerter is a test-only Alerter that increments a counter
// and records the WI it was dispatched for. Used to prove the
// Registry routes to the right strategy.
type recordingAlerter struct {
	name string
	hits atomic.Int64
	last atomic.Pointer[model.WorkItem]
}

func (r *recordingAlerter) Name() string { return r.name }
func (r *recordingAlerter) Dispatch(_ context.Context, wi model.WorkItem) error {
	r.hits.Add(1)
	r.last.Store(&wi)
	return nil
}

func wiWith(severity model.Severity) model.WorkItem {
	return model.WorkItem{
		ID:            uuid.New(),
		ComponentID:   "X",
		ComponentType: model.ComponentCache,
		Severity:      severity,
		Status:        model.StatusOpen,
	}
}

// TestRegistry_RoutesBySeverity: build the v1 registry (PagerDuty for
// P0, Slack for P1/P2, Console for P3) and verify each severity goes
// to the right alerter.
func TestRegistry_RoutesBySeverity(t *testing.T) {
	pd := &recordingAlerter{name: "pagerduty_stub"}
	slack := &recordingAlerter{name: "slack_webhook"}
	console := &recordingAlerter{name: "console"}

	reg := NewRegistry(
		map[string]Alerter{
			"pagerduty_stub": pd,
			"slack_webhook":  slack,
			"console":        console,
		},
		[]Rule{
			SeverityRule("p0", "pagerduty_stub", model.SeverityP0),
			SeverityRule("p12", "slack_webhook", model.SeverityP1, model.SeverityP2),
		},
	)

	cases := []struct {
		sev  model.Severity
		want Alerter
	}{
		{model.SeverityP0, pd},
		{model.SeverityP1, slack},
		{model.SeverityP2, slack},
		{model.SeverityP3, console}, // falls through to fallback
	}
	for _, tc := range cases {
		got := reg.ForWorkItem(wiWith(tc.sev))
		if got != tc.want {
			t.Errorf("%s: routed to %v (%s), want %v", tc.sev, got, got.Name(), tc.want)
		}
	}
}

// TestRegistry_FallsBackToConsoleWhenNoMatch: if no rule matches, the
// Registry hands back the registered "console" alerter so the system
// is never completely silent.
func TestRegistry_FallsBackToConsoleWhenNoMatch(t *testing.T) {
	console := &recordingAlerter{name: "console"}
	reg := NewRegistry(
		map[string]Alerter{"console": console},
		nil, // zero rules; every WI falls through
	)

	got := reg.ForWorkItem(wiWith(model.SeverityP0))
	if got != console {
		t.Errorf("want console fallback, got %v", got)
	}
}

// TestConsoleAlerter_NoOpDispatch: ConsoleAlerter.Dispatch never errors.
func TestConsoleAlerter_NoOpDispatch(t *testing.T) {
	if err := (ConsoleAlerter{}).Dispatch(context.Background(), wiWith(model.SeverityP3)); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

// TestPagerDutyStub_NoOpDispatch: stub never errors.
func TestPagerDutyStub_NoOpDispatch(t *testing.T) {
	if err := (PagerDutyStub{}).Dispatch(context.Background(), wiWith(model.SeverityP0)); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

// TestSlackAlerter_PostsToWebhook: spin up an httptest server, point
// the alerter at it, dispatch, and verify the JSON shape Slack would
// receive.
func TestSlackAlerter_PostsToWebhook(t *testing.T) {
	var got slackPayload
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	a := NewSlackAlerter(srv.URL, time.Second)
	err := a.Dispatch(context.Background(), wiWith(model.SeverityP1))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if hits.Load() != 1 {
		t.Errorf("want 1 webhook call, got %d", hits.Load())
	}
	if got.Text == "" {
		t.Errorf("payload.text empty; got %+v", got)
	}
}

// TestSlackAlerter_EmptyURL_NoOps: if SLACK_WEBHOOK_URL is unset, the
// alerter does nothing (the registry should have wired in ConsoleAlerter
// instead, but defensive no-op beats a nil-URL panic).
func TestSlackAlerter_EmptyURL_NoOps(t *testing.T) {
	a := NewSlackAlerter("", time.Second)
	if err := a.Dispatch(context.Background(), wiWith(model.SeverityP1)); err != nil {
		t.Errorf("empty URL: want nil error, got %v", err)
	}
}

// TestSlackAlerter_HonoursContextTimeout: slow webhook + a tight ctx
// deadline → return error within budget (not block forever). FR-6.4.
func TestSlackAlerter_HonoursContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // longer than our ctx deadline
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	a := NewSlackAlerter(srv.URL, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := a.Dispatch(ctx, wiWith(model.SeverityP1))
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("expected timeout error, got nil")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("dispatch took %v; should bail near 50ms ctx deadline", elapsed)
	}
}

// TestSlackAlerter_NonSuccessStatus_ReturnsError: a 500 from the
// webhook surfaces as an error.
func TestSlackAlerter_NonSuccessStatus_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	a := NewSlackAlerter(srv.URL, time.Second)
	err := a.Dispatch(context.Background(), wiWith(model.SeverityP2))
	if err == nil {
		t.Fatal("expected error from 500 response")
	}
}
