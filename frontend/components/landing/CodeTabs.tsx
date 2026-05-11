// LANDING.md §5.7 — live code tabs.
//
// The most important credibility section on the page. Real code from
// the actual backend, syntax-highlighted with the custom ims-dark
// Shiki theme (see lib/highlight.ts).
//
// Architecture:
//   * This file is a Server Component that pre-renders the highlighted
//     HTML at build time for all four tabs.
//   * It hands the rendered HTML strings to <CodeTabsClient> which
//     manages the tab-switch state on the client. Zero Shiki runtime
//     ships to the browser.
//
// Tabs (in spec order):
//   1. Ingestion        — POST /v1/signals non-blocking enqueue
//   2. Debounce         — the Lua atomic script
//   3. State machine    — ResolvedState.CanTransitionTo (the RCA rule)
//   4. Alerter          — Registry.ForWorkItem strategy pattern

import { highlight } from "@/lib/highlight";

import { CodeTabsClient, type Tab } from "./CodeTabsClient";

// ---- Real source snippets ----
//
// These are LITERALLY copy-pasted from the repo. Don't edit them
// inline — re-paste from the file when the backend changes.
// The file-path comments in the prose must match these snippets'
// actual file paths.

const INGESTION_CODE = `// backend/internal/ingest/handler.go
// THE HOT PATH. Single non-blocking enqueue. Do NOT add work here.
if !p.Submit(sig) {
    c.Header("Retry-After", "1")
    c.JSON(http.StatusServiceUnavailable, gin.H{
        "error":          "ingestion queue full",
        "retry_after_ms": retryAfterMillis,
    })
    return
}
c.JSON(http.StatusAccepted, gin.H{
    "status":    "accepted",
    "signal_id": sig.SignalID,
})

// backend/internal/pipeline/pipeline.go
func (p *Pipeline) Submit(sig model.Signal) bool {
    select {
    case p.queue <- sig:
        p.accepted.Add(1)
        return true
    default:
        p.dropped.Add(1)
        return false
    }
}`;

const DEBOUNCE_CODE = `-- backend/internal/debounce/script.lua
-- Atomic "join or create" decision for one component.
-- Redis runs Lua scripts single-threaded server-side, so the
-- entire check-then-act is atomic across all clients —
-- eliminates the race without a distributed lock.

local existing = redis.call('GET', KEYS[1])
local count = tonumber(redis.call('GET', KEYS[2]) or '0')

if existing and count < tonumber(ARGV[3]) then
    redis.call('INCR', KEYS[2])
    return {existing, 'JOINED', count + 1}
else
    redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
    redis.call('SET', KEYS[2], '1', 'EX', ARGV[2])
    return {ARGV[1], 'CREATED', 1}
end`;

const STATE_MACHINE_CODE = `// backend/internal/workflow/state.go
// THE RULE: "CLOSED requires an RCA" lives in exactly one place.
// Any path that closes a Work Item routes through here.
func (ResolvedState) CanTransitionTo(next State, tctx TransitionContext) error {
    if _, ok := next.(ClosedState); !ok {
        return fmt.Errorf("%w: RESOLVED -> %s", ErrInvalidTransition, next.Name())
    }
    if tctx.RCA == nil {
        return ErrMissingRCA
    }
    if errs := tctx.RCA.Validate(); len(errs) > 0 {
        return &IncompleteRCAError{Fields: errs}
    }
    return nil
}`;

const ALERTER_CODE = `// backend/internal/alert/alerter.go
// Strategy pattern. Adding a new channel (Teams, OpsGenie) is one
// new struct + one new Rule. Zero changes to the processor.
func (r *Registry) ForWorkItem(wi model.WorkItem) Alerter {
    for _, rule := range r.rules {
        if rule.Matches(wi) {
            if a, ok := r.alerters[rule.AlerterName]; ok {
                return a
            }
        }
    }
    return r.alerters["console"]
}`;

// ---- Tab content ----

const TAB_PROSE: Record<string, { p1: string; p2: string; href: string }> = {
  ingestion: {
    p1: "The handler accepts a signal, validates it, and pushes it onto a bounded Go channel. One non-blocking select — no DB call, no lock, no work happens on the hot path.",
    p2: "When the channel is full we return 503 with a Retry-After header. That's the entire backpressure story. The pipeline's Submit method is the single point where signals enter the system.",
    href: "https://github.com/kubeboiii/ims/blob/main/backend/internal/ingest/handler.go",
  },
  debounce: {
    p1: "One Lua script collapses up to 100 correlated signals into a single Work Item. Redis runs Lua server-side, single-threaded — so the check-then-act is atomic across every ingestion worker.",
    p2: "No distributed lock. No Redlock edge cases. If Redis goes down, the system degrades to always-CREATE mode and keeps accepting signals. When Redis comes back, debouncing resumes on the next call.",
    href: "https://github.com/kubeboiii/ims/blob/main/backend/internal/debounce/script.lua",
  },
  "state-machine": {
    p1: "The State pattern encodes the OPEN → INVESTIGATING → RESOLVED → CLOSED lifecycle. Every state-specific rule lives on its own type. There's no switch statement.",
    p2: "The rule that ResolvedState can only transition to CLOSED with a valid RCA lives in exactly one method. A reviewer asking 'where do we enforce the RCA rule?' finds it in five seconds.",
    href: "https://github.com/kubeboiii/ims/blob/main/backend/internal/workflow/state.go",
  },
  alerter: {
    p1: "The Strategy pattern routes each new Work Item to the right alerter based on severity. PagerDuty stub for P0, Slack webhook for P1/P2, Console for the rest.",
    p2: "Adding a new channel — Microsoft Teams, OpsGenie, whatever — is one new struct that implements the Alerter interface plus one new Rule. The processor, the workflow engine, and the persistence layer don't change.",
    href: "https://github.com/kubeboiii/ims/blob/main/backend/internal/alert/alerter.go",
  },
};

export async function CodeTabs() {
  // Pre-render all four snippets to HTML at build time. Highlights
  // happen on the server (Node), not in the browser.
  const [ingHtml, debHtml, stateHtml, alertHtml] = await Promise.all([
    highlight(INGESTION_CODE, "go"),
    highlight(DEBOUNCE_CODE, "lua"),
    highlight(STATE_MACHINE_CODE, "go"),
    highlight(ALERTER_CODE, "go"),
  ]);

  const tabs: Tab[] = [
    { id: "ingestion", label: "Ingestion", codeHtml: ingHtml, ...TAB_PROSE.ingestion },
    { id: "debounce", label: "Debounce", codeHtml: debHtml, ...TAB_PROSE.debounce },
    { id: "state-machine", label: "State machine", codeHtml: stateHtml, ...TAB_PROSE["state-machine"] },
    { id: "alerter", label: "Alerter", codeHtml: alertHtml, ...TAB_PROSE.alerter },
  ];

  return (
    <section className="border-t border-divider px-6 py-24 sm:py-32">
      <div className="mx-auto max-w-[1120px]">
        <header className="mb-10">
          <p className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            Real code
          </p>
          <h2 className="mt-3 font-sans text-[28px] font-medium leading-[1.2] text-text-primary sm:text-[36px]">
            Pulled straight from the repo.
          </h2>
        </header>

        <CodeTabsClient tabs={tabs} />
      </div>
    </section>
  );
}
