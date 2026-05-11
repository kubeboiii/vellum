// Phase 6 — headless failure simulator.
//
// Fires three pre-canned outage scenarios at the IMS backend's
// /v1/signals endpoint and prints a one-page summary showing the
// debounce compression ratio per scenario.
//
// Usage:
//
//	go run ./scripts/simulate-outage.go                       # all scenarios
//	go run ./scripts/simulate-outage.go --scenario rdbms      # just one
//	go run ./scripts/simulate-outage.go --target http://...   # different backend
//
// Scenarios:
//   - rdbms : 50 P0 to RDBMS_PRIMARY_01, then 100 P1 to API_CHECKOUT
//   - cache : 200 P2 to CACHE_CLUSTER_A
//   - mcp   : 30 P0 to MCP_HOST_INDEXER + 80 P1 fanned over 4 APIs
//   - all   : runs all three sequentially (default)
//
// The script tolerates 503 (backpressure) and 5xx (other) — these
// are counted and reported, not fatal. Successful runs print a
// debounce ratio per scenario so PRD G2 (≥60×) is verifiable
// from a clean checkout.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// ─── scenario model ──────────────────────────────────────────────────

type step struct {
	count         int
	rps           int
	componentID   string
	componentType string
	severity      string
}

type scenario struct {
	id    string
	title string
	steps []step
}

func scenarios() []scenario {
	return []scenario{
		{
			id:    "rdbms",
			title: "RDBMS cascade",
			steps: []step{
				{50, 25, "RDBMS_PRIMARY_01", "RDBMS", "P0"},
				{100, 50, "API_CHECKOUT", "API", "P1"},
			},
		},
		{
			id:    "cache",
			title: "Cache thrash",
			steps: []step{
				{200, 20, "CACHE_CLUSTER_A", "CACHE", "P2"},
			},
		},
		{
			id:    "mcp",
			title: "MCP host fail",
			steps: []step{
				{30, 15, "MCP_HOST_INDEXER", "MCP_HOST", "P0"},
				{20, 20, "API_SEARCH", "API", "P1"},
				{20, 20, "API_RECOMMEND", "API", "P1"},
				{20, 20, "API_HOMEFEED", "API", "P1"},
				{20, 20, "API_NOTIFICATIONS", "API", "P1"},
			},
		},
	}
}

// ─── per-scenario counters ───────────────────────────────────────────

type counters struct {
	sent     atomic.Int64
	accepted atomic.Int64
	rejected atomic.Int64 // 503 specifically
	failed   atomic.Int64 // anything else non-2xx
}

type runResult struct {
	scenario   scenario
	counters   *counters
	duration   time.Duration
	workItems  int
	components map[string]int // component_id -> work_item count
}

// ─── main ────────────────────────────────────────────────────────────

func main() {
	target := flag.String("target", "http://localhost:8080", "IMS backend base URL")
	pick := flag.String("scenario", "all", "rdbms | cache | mcp | all")
	flag.Parse()

	all := scenarios()
	var toRun []scenario
	switch *pick {
	case "all":
		toRun = all
	default:
		for _, s := range all {
			if s.id == *pick {
				toRun = []scenario{s}
			}
		}
		if toRun == nil {
			fmt.Fprintf(os.Stderr, "unknown scenario %q; want one of: rdbms|cache|mcp|all\n", *pick)
			os.Exit(2)
		}
	}

	// Sanity: backend reachable?
	if !ping(*target) {
		fmt.Fprintf(os.Stderr, "❌ %s/health is unreachable. Is the backend running?\n", *target)
		os.Exit(1)
	}
	fmt.Printf("▶ target: %s\n", *target)
	fmt.Println()

	results := make([]runResult, 0, len(toRun))
	for _, sc := range toRun {
		r := runScenario(*target, sc)
		results = append(results, r)
		printScenarioReport(r)
		fmt.Println()
	}

	if len(results) > 1 {
		printAggregateReport(results)
	}
}

// ─── scenario execution ──────────────────────────────────────────────

func runScenario(target string, sc scenario) runResult {
	c := &counters{}
	start := time.Now()

	// Snapshot the existing work_items for the components we're
	// about to touch, so we can compute the delta accurately.
	preExisting := countWorkItemsForComponents(target, componentSet(sc))

	var wg sync.WaitGroup
	for _, st := range sc.steps {
		wg.Add(1)
		go func(st step) {
			defer wg.Done()
			runStep(target, st, c)
		}(st)
	}
	wg.Wait()

	// Give the workers a beat to flush. The pipeline is async; if
	// we GET /v1/incidents immediately, the last few signals may
	// not yet have their work_items materialised in Postgres.
	time.Sleep(500 * time.Millisecond)

	after := countWorkItemsForComponents(target, componentSet(sc))
	delta := map[string]int{}
	for k, v := range after {
		delta[k] = v - preExisting[k]
	}
	totalNew := 0
	for _, n := range delta {
		totalNew += n
	}
	return runResult{
		scenario:   sc,
		counters:   c,
		duration:   time.Since(start),
		workItems:  totalNew,
		components: delta,
	}
}

func runStep(target string, st step, c *counters) {
	interval := time.Second / time.Duration(st.rps)
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var wg sync.WaitGroup
	for i := 0; i < st.count; i++ {
		<-ticker.C
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			postOne(target, st, i, c)
		}(i)
	}
	wg.Wait()
}

func postOne(target string, st step, i int, c *counters) {
	c.sent.Add(1)
	body, _ := json.Marshal(map[string]any{
		"component_id":   st.componentID,
		"component_type": st.componentType,
		"severity":       st.severity,
		"source":         "simulate-outage",
		"payload":        map[string]any{"i": i, "via": "scripts/simulate-outage.go"},
	})
	req, _ := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		target+"/v1/signals",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.failed.Add(1)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	switch {
	case resp.StatusCode == http.StatusAccepted, resp.StatusCode == http.StatusOK:
		c.accepted.Add(1)
	case resp.StatusCode == http.StatusServiceUnavailable:
		c.rejected.Add(1)
	default:
		c.failed.Add(1)
	}
}

// ─── reporting ───────────────────────────────────────────────────────

func printScenarioReport(r runResult) {
	sc := r.scenario
	c := r.counters
	expected := expectedWorkItems(sc) // ceil(Σcount per component / 100)
	accepted := c.accepted.Load()
	ratio := "—"
	if r.workItems > 0 {
		ratio = fmt.Sprintf("%.1f×", float64(accepted)/float64(r.workItems))
	}

	fmt.Printf("█ %s\n", sc.title)
	fmt.Printf("  steps:\n")
	for _, st := range sc.steps {
		fmt.Printf("    %-3s %-22s × %d @ %d/s\n", st.severity, st.componentID, st.count, st.rps)
	}
	fmt.Printf("  sent:        %d\n", c.sent.Load())
	fmt.Printf("  accepted:    %d  (%s)\n", accepted, pct(accepted, c.sent.Load()))
	fmt.Printf("  rejected:    %d  (503/backpressure)\n", c.rejected.Load())
	fmt.Printf("  failed:      %d  (other errors)\n", c.failed.Load())
	fmt.Printf("  duration:    %s\n", r.duration.Round(10*time.Millisecond))
	fmt.Printf("  work items:  %d created  (expected %d)\n", r.workItems, expected)
	fmt.Printf("  ratio:       %s debounce compression\n", ratio)
	if r.workItems == expected {
		fmt.Printf("  ✓ debounce held exactly as predicted\n")
	} else if r.workItems > 0 {
		fmt.Printf("  ! work_item count differs from prediction (timing-dependent)\n")
	}
}

func printAggregateReport(results []runResult) {
	var sent, accepted, rejected, failed, items, expected int64
	for _, r := range results {
		sent += r.counters.sent.Load()
		accepted += r.counters.accepted.Load()
		rejected += r.counters.rejected.Load()
		failed += r.counters.failed.Load()
		items += int64(r.workItems)
		expected += int64(expectedWorkItems(r.scenario))
	}
	ratio := "—"
	if items > 0 {
		ratio = fmt.Sprintf("%.1f×", float64(accepted)/float64(items))
	}
	fmt.Println("═══════════════════════════════════════════")
	fmt.Println(" AGGREGATE")
	fmt.Println("═══════════════════════════════════════════")
	fmt.Printf("  scenarios:   %d\n", len(results))
	fmt.Printf("  sent:        %d\n", sent)
	fmt.Printf("  accepted:    %d  (%s)\n", accepted, pct(accepted, sent))
	fmt.Printf("  rejected:    %d\n", rejected)
	fmt.Printf("  failed:      %d\n", failed)
	fmt.Printf("  work items:  %d created  (expected %d)\n", items, expected)
	fmt.Printf("  ratio:       %s overall debounce compression\n", ratio)
	fmt.Println()
	if ratio != "—" && accepted >= 60*items {
		fmt.Println("✓ PRD G2 SATISFIED: ratio ≥ 60×")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────

func ping(target string) bool {
	resp, err := http.Get(target + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

type workItemListItem struct {
	ID          string `json:"id"`
	ComponentID string `json:"component_id"`
}
type workItemList struct {
	Items []workItemListItem `json:"items"`
}

// countWorkItemsForComponents returns a map component_id -> count
// of work_items currently in the active list for the components we
// care about. Used to compute deltas before/after a scenario so
// rerunning on the same DB still shows accurate "new" counts.
func countWorkItemsForComponents(target string, ids map[string]bool) map[string]int {
	out := map[string]int{}
	resp, err := http.Get(target + "/v1/incidents?limit=500")
	if err != nil {
		return out
	}
	defer resp.Body.Close()
	var raw workItemList
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return out
	}
	for _, wi := range raw.Items {
		if ids[wi.ComponentID] {
			out[wi.ComponentID]++
		}
	}
	return out
}

func componentSet(sc scenario) map[string]bool {
	out := map[string]bool{}
	for _, st := range sc.steps {
		out[st.componentID] = true
	}
	return out
}

// expectedWorkItems applies FR-3.1: each component_id's window
// holds at most 100 signals. So expected = sum_per_component(
// ceil(total_count_for_that_component / 100) ).
func expectedWorkItems(sc scenario) int {
	totals := map[string]int{}
	for _, st := range sc.steps {
		totals[st.componentID] += st.count
	}
	total := 0
	for _, n := range totals {
		total += (n + 99) / 100 // integer ceil
	}
	return total
}

func pct(num, den int64) string {
	if den == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(num)/float64(den))
}
