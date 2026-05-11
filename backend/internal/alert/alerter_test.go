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

	"github.com/kubeboiii/vellum/internal/model"
)

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
		{model.SeverityP3, console},
	}
	for _, tc := range cases {
		got := reg.ForWorkItem(wiWith(tc.sev))
		if got != tc.want {
			t.Errorf("%s: routed to %v (%s), want %v", tc.sev, got, got.Name(), tc.want)
		}
	}
}

func TestRegistry_FallsBackToConsoleWhenNoMatch(t *testing.T) {
	console := &recordingAlerter{name: "console"}
	reg := NewRegistry(
		map[string]Alerter{"console": console},
		nil,
	)

	got := reg.ForWorkItem(wiWith(model.SeverityP0))
	if got != console {
		t.Errorf("want console fallback, got %v", got)
	}
}

func TestConsoleAlerter_NoOpDispatch(t *testing.T) {
	if err := (ConsoleAlerter{}).Dispatch(context.Background(), wiWith(model.SeverityP3)); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

func TestPagerDutyStub_NoOpDispatch(t *testing.T) {
	if err := (PagerDutyStub{}).Dispatch(context.Background(), wiWith(model.SeverityP0)); err != nil {
		t.Errorf("unexpected: %v", err)
	}
}

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

func TestSlackAlerter_EmptyURL_NoOps(t *testing.T) {
	a := NewSlackAlerter("", time.Second)
	if err := a.Dispatch(context.Background(), wiWith(model.SeverityP1)); err != nil {
		t.Errorf("empty URL: want nil error, got %v", err)
	}
}

func TestSlackAlerter_HonoursContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
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
