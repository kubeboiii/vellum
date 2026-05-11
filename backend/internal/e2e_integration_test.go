//go:build integration

package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	defaultBaseURL = "http://localhost:8080"
	pollTimeout    = 5 * time.Second
	pollInterval   = 200 * time.Millisecond
)

func baseURL() string {
	if v := os.Getenv("VELLUM_BASE_URL"); v != "" {
		return v
	}
	return defaultBaseURL
}

func TestE2E_FullLifecycle(t *testing.T) {
	componentID := fmt.Sprintf("E2E_%d", time.Now().UnixNano())
	t.Logf("component_id: %s", componentID)

	const signalCount = 3
	for i := 0; i < signalCount; i++ {
		body := map[string]any{
			"component_id":   componentID,
			"component_type": "API",
			"severity":       "P1",
			"source":         "e2e-integration-test",
			"payload":        map[string]any{"i": i, "test": "phase-6"},
		}
		mustPOST(t, "/v1/signals", body, http.StatusAccepted)
	}

	wiID := waitForIncident(t, componentID)

	wi := mustGetIncident(t, wiID)
	if wi.Status != "OPEN" {
		t.Fatalf("new work_item should be OPEN, got %s", wi.Status)
	}
	if wi.SignalCount != signalCount {

		t.Fatalf("expected signal_count=%d, got %d", signalCount, wi.SignalCount)
	}

	signals := mustGetSignals(t, wiID)
	if signals.Total != signalCount {
		t.Fatalf("expected %d raw signals in Mongo, got %d", signalCount, signals.Total)
	}

	mustPatchState(t, wiID, "INVESTIGATING", http.StatusOK)
	mustPatchState(t, wiID, "RESOLVED", http.StatusOK)

	resp, body := patchState(t, wiID, "CLOSED")
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("RCA gate should reject CLOSED without RCA with 422; got %d (body=%s)",
			resp.StatusCode, body)
	}
	if !strings.Contains(strings.ToLower(body), "rca") {
		t.Fatalf("422 body should mention rca; got %s", body)
	}

	rca := map[string]any{
		"root_cause_category": "CODE_DEFECT",
		"fix_applied":         "reverted bad deploy; added boundary-case regression test",
		"prevention_steps":    "add lint rule to flag unhandled null returns at module boundaries",
		"submitted_by":        "e2e-integration-test",
	}
	mustPOST(t, fmt.Sprintf("/v1/incidents/%s/rca", wiID), rca, http.StatusCreated)

	final := mustGetIncident(t, wiID)
	if final.Status != "CLOSED" {
		t.Fatalf("after RCA, work_item should be CLOSED; got %s", final.Status)
	}
	if final.MTTRSeconds == nil || *final.MTTRSeconds < 0 {
		t.Fatalf("MTTR should be computed and >= 0; got %+v", final.MTTRSeconds)
	}
	if final.ClosedAt == nil || *final.ClosedAt == "" {
		t.Fatalf("closed_at should be set; got %+v", final.ClosedAt)
	}

	trs := mustGetTransitions(t, wiID)
	if trs.Count < 3 {
		t.Fatalf("expected at least 3 transitions in audit; got %d", trs.Count)
	}

	want := []struct{ from, to string }{
		{"OPEN", "INVESTIGATING"},
		{"INVESTIGATING", "RESOLVED"},
		{"RESOLVED", "CLOSED"},
	}
	for i, w := range want {
		got := trs.Items[i]
		if got.FromState != w.from || got.ToState != w.to {
			t.Fatalf("transition %d: want %s->%s, got %s->%s",
				i, w.from, w.to, got.FromState, got.ToState)
		}
	}
}

type workItem struct {
	ID          string  `json:"id"`
	ComponentID string  `json:"component_id"`
	Status      string  `json:"status"`
	SignalCount int     `json:"signal_count"`
	MTTRSeconds *int    `json:"mttr_seconds,omitempty"`
	ClosedAt    *string `json:"closed_at,omitempty"`
}

type incidentDetailResp struct {
	WorkItem workItem `json:"work_item"`
}

type incidentsListResp struct {
	Items []workItem `json:"items"`
	Count int        `json:"count"`
}

type signalsPageResp struct {
	Total int `json:"total"`
}

type stateTransition struct {
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
}

type transitionsResp struct {
	Items []stateTransition `json:"items"`
	Count int               `json:"count"`
}

func mustPOST(t *testing.T, path string, body any, wantStatus int) {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, baseURL()+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s: want %d got %d (body=%s)", path, wantStatus, resp.StatusCode, string(raw))
	}
}

func patchState(t *testing.T, id string, to string) (*http.Response, string) {
	t.Helper()
	b, _ := json.Marshal(map[string]any{
		"to":     to,
		"actor":  "e2e",
		"reason": "integration test",
	})
	req, _ := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/v1/incidents/%s/state", baseURL(), id),
		bytes.NewReader(b),
	)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH state: %v", err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(raw)
}

func mustPatchState(t *testing.T, id string, to string, wantStatus int) {
	t.Helper()
	resp, body := patchState(t, id, to)
	if resp.StatusCode != wantStatus {
		t.Fatalf("PATCH state -> %s: want %d got %d (body=%s)",
			to, wantStatus, resp.StatusCode, body)
	}
}

func mustGetIncident(t *testing.T, id string) workItem {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/v1/incidents/%s", baseURL(), id))
	if err != nil {
		t.Fatalf("GET incident: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET incident: status %d", resp.StatusCode)
	}
	var out incidentDetailResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode incident: %v", err)
	}
	return out.WorkItem
}

func mustGetSignals(t *testing.T, id string) signalsPageResp {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/v1/incidents/%s/signals", baseURL(), id))
	if err != nil {
		t.Fatalf("GET signals: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET signals: status %d", resp.StatusCode)
	}
	var out signalsPageResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode signals: %v", err)
	}
	return out
}

func mustGetTransitions(t *testing.T, id string) transitionsResp {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/v1/incidents/%s/transitions", baseURL(), id))
	if err != nil {
		t.Fatalf("GET transitions: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET transitions: status %d", resp.StatusCode)
	}
	var out transitionsResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode transitions: %v", err)
	}
	return out
}

func waitForIncident(t *testing.T, componentID string) string {
	t.Helper()
	deadline := time.Now().Add(pollTimeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/v1/incidents?limit=500", baseURL()))
		if err == nil && resp.StatusCode == http.StatusOK {
			var out incidentsListResp
			_ = json.NewDecoder(resp.Body).Decode(&out)
			resp.Body.Close()
			for _, wi := range out.Items {
				if wi.ComponentID == componentID {
					return wi.ID
				}
			}
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("incident for component %s never appeared within %s", componentID, pollTimeout)
	return ""
}
