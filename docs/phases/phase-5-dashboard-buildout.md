# Phase 5 — Dashboard Buildout (Tier 1)

> **Document:** `docs/phases/phase-5-dashboard-buildout.md`
> **Owner:** Frontend — Phase 5 polish
> **Created:** 2026-05-11
> **Status:** In progress

## 0. Purpose

This is the master plan for the Tier-1 dashboard expansion approved by the
user. Every item below is buildable against the **existing backend endpoints**
(see `03-api-contract` / `lib/api.ts`). **No backend changes. No new
dependencies. No gradients.**

The plan exists so I can build all of it in one pass without forgetting
anything, without breaking design consistency, and without merging things
that don't compose.

## 1. Tech constraints (non-negotiable)

| Constraint | Source | Implication |
|---|---|---|
| `gofmt`-clean Go, App Router only, Tailwind utility-first | `CLAUDE.md` | No Pages Router pages, no CSS files |
| No gradients, no glassmorphism | `THEME.md §8.4` | Halos are `box-shadow`, never `linear-gradient` |
| All motion gated on `prefers-reduced-motion` | landing convention | Every framer-motion `animate` must short-circuit when `useReducedMotion()` is true |
| SSR-safe (no `Math.random` in initial render) | LogTape bug fix | Seeded initial state, jitter applied in `useEffect` |
| Severity palette stays: P0 red, P1 amber, P2 lime, P3 grey | `THEME.md` | Reuse `bg-sev-p0` / `bg-sev-p1` / `bg-sev-p2` / `bg-sev-p3` |
| Annotation violet is a landing-only token | `globals.css` | Use sparingly on the dashboard; severity tokens preferred |
| New types must mirror Go JSON tags | `lib/types.ts` header | Don't invent fields — derive from the existing types |

## 2. Files I will create or modify (catalog)

### New components (under `frontend/components/`)

| File | Purpose | Used by |
|---|---|---|
| `dashboard/SeverityStackedBar.tsx` | Single horizontal bar showing P0/P1/P2/P3 mix; hover for counts | `/dashboard` |
| `dashboard/NoisyComponents.tsx` | Top-5 components ranked by `signal_count`, severity-colored bars | `/dashboard` |
| `dashboard/IncidentRateStrip.tsx` | 5 stacked sparklines, one per minute, of new-incident rate | `/dashboard` |
| `dashboard/PersonaSwitcher.tsx` | 3-way pill toggle: SRE / Commander / Post-mortem author | `/dashboard` |
| `dashboard/HealthStrip.tsx` | Polls `/health`, renders dep chips + latency + queue gauge | `Nav` (top) |
| `dashboard/QueueGauge.tsx` | Horizontal bar, depth/capacity, pulses amber→red | inside `HealthStrip` |
| `dashboard/SignalFrequency.tsx` | Time-bucket histogram of an incident's signals | `/incidents/[id]` |
| `dashboard/PayloadFingerprints.tsx` | Group signals by hashed payload-key shape | `/incidents/[id]` |
| `dashboard/IncidentSeverityOverTime.tsx` | Severity distribution along the incident's time axis | `/incidents/[id]` |
| `dashboard/TransitionTimeline.tsx` | Horizontal track with one tick per state transition | `/incidents/[id]` |
| `dashboard/TimeInState.tsx` | Bar showing time spent in each state | `/incidents/[id]` |
| `dashboard/MTTRHistogram.tsx` | Bucketed histogram of `mttr_seconds` | `/incidents/closed` |
| `dashboard/CategoryBreakdown.tsx` | Donut / stacked bar of `root_cause_category` | `/incidents/closed` |
| `dashboard/MTTRTrend.tsx` | Line chart of MTTR per closed incident over time | `/incidents/closed` |
| `dashboard/RepeatOffenders.tsx` | Group closed incidents by `component_id`; show count + avg MTTR | `/incidents/closed` |
| `dashboard/CornerTicks.tsx` | Shared L-mark corner ticks (extracted; used by `/load-test`, `/simulate`) | new pages |
| `dashboard/SoundAlerts.tsx` (or hook) | Beep on new P0 when Nav unmute is on | `/dashboard` |

### New pages (under `frontend/app/`)

| Path | Purpose | Persona |
|---|---|---|
| `postmortem/page.tsx` | RESOLVED-waiting-for-RCA queue + RCA quality stats | Post-mortem author |
| `load-test/page.tsx` | Burst N synthetic signals; live counters tick | Demo |
| `simulate/page.tsx` | Pre-canned failure scenario buttons; observe end-to-end | Demo |
| `flow/page.tsx` | "How a signal flows" interactive diagram (Tier 1 item 21) | Demo |

### Modified files

| File | Change |
|---|---|
| `app/dashboard/page.tsx` | Wire new widgets in; replace 4 StatCards with the new stacked bar **plus** keep the existing four cards (they earn their keep) below |
| `app/incidents/[id]/page.tsx` | Add SignalFrequency, PayloadFingerprints, IncidentSeverityOverTime, TransitionTimeline, TimeInState |
| `app/incidents/closed/page.tsx` | Add MTTRHistogram, CategoryBreakdown, MTTRTrend, RepeatOffenders at the top |
| `components/Nav.tsx` | Add link to /postmortem, /load-test, /simulate, /flow; mount HealthStrip below nav |
| `lib/api.ts` | Add `getHealth()`, `listAllSignalsForIncident()` (paged helper for histograms) |
| `lib/types.ts` | Add `Health`, `Dependency` interfaces matching `/health` JSON |
| `lib/persona.ts` (new) | Persona enum + helpers for PersonaSwitcher |
| `lib/fingerprint.ts` (new) | Pure function: payload → top-key-shape hash |

## 3. Design language (must not drift)

Every new component picks from this kit. **No new visual idioms.**

| Idiom | Tailwind / utility |
|---|---|
| Section card | `rounded-md border border-border-subtle bg-bg-surface` |
| Section card hover | `transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong` + inline lime/sev `boxShadow` on mouseenter |
| Section header | `relative pb-2 font-sans text-section font-semibold text-text-primary` with `<span class="absolute -bottom-1 left-0 h-px w-6 bg-accent" />` |
| Big numbers | `font-mono text-stat font-medium tabular-nums` |
| Labels | `font-mono text-label uppercase tracking-[0.05em] text-text-secondary` |
| Live dot | `h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent` (or `bg-sev-p0` for severe) |
| Count-up animation | `easeOutQuart`, 800–900ms, `useInView` trigger |
| Hover glow | `box-shadow: 0 0 28px -10px <color>` (lime/red/violet/grey) |
| Severity colors | `text-sev-p0` / `bg-sev-p0` etc. — never raw red hex in className |
| Severity halo (incident-row hover) | matches `SEV_GLOW` already in `IncidentRow.tsx` — extract |
| Corner ticks | extract `CornerTicks.tsx` from `ProblemStrip.tsx` for reuse |
| Drifting grid | reuse `GridTexture` idiom; only on hero-style pages, not data dense |

## 4. Build order (dependency-respecting)

1. **Shared infra** — types + api helpers + fingerprint lib + persona lib + extracted CornerTicks
2. **HealthStrip + QueueGauge** — mounted in Nav so it sees every page
3. **Section A widgets** (SeverityStackedBar, NoisyComponents, IncidentRateStrip, PersonaSwitcher, SoundAlerts) and wire `/dashboard`
4. **Section B widgets** (SignalFrequency, PayloadFingerprints, IncidentSeverityOverTime, TransitionTimeline, TimeInState) and wire `/incidents/[id]`
5. **Section C widgets** (MTTRHistogram, CategoryBreakdown, MTTRTrend, RepeatOffenders) and wire `/incidents/closed`
6. **Section D** new `/postmortem` page
7. **Section E** new `/load-test`, `/simulate`, `/flow` pages
8. **Nav update** — add links to all new routes
9. **Build + route smoke test** — `pnpm build`, hit every route, assert 200, restart prod server

## 5. Per-component specifications

### A1. `SeverityStackedBar`
- Props: `items: WorkItem[]`
- Compute counts per severity (P0..P3). Render a single horizontal flex bar where each segment's `flex-basis` = `(count / total) * 100%`.
- Empty state (`total === 0`): one neutral grey bar with "No active incidents".
- Each segment uses `bg-sev-pN`, height 6px, rounded-sm container.
- Above the bar: 4 counter chips with severity dot + number; the active severity (if any P0 exists, highlight P0) gets a pulsing live dot.
- Hover a segment: lift to 8px, tooltip with "P0 · 3 incidents · 60%".

### A2. `NoisyComponents`
- Props: `items: WorkItem[]`
- Pick top-5 by `signal_count` desc. For each: component_id, bar width proportional to signal count vs. max; bar color = severity color.
- Component ID truncated at 22 chars with ellipsis; signal count tabular-nums on the right.
- Click → navigate to `/incidents/{id}` (top-incident-for-that-component).

### A3. `IncidentRateStrip`
- Props: `items: WorkItem[]` and a rolling history of `{minute, count}` buckets passed from the dashboard (parent already has `buckets` state).
- Render the *last 5 minutes* as five tiny sparklines stacked vertically (or one row of five), each with a label "−4m, −3m, −2m, −1m, now".
- Use recharts `LineChart` already imported.

### A4. `PersonaSwitcher`
- Pure client. Pill toggle: `SRE` (default) / `Commander` / `Post-mortem`.
- Stored in `useState`; persist via `localStorage` key `vellum.persona`.
- Emits an `onChange(persona)`.
- Each persona changes:
  - SRE → live feed sort: severity DESC then last_signal_ts DESC (current behavior)
  - Commander → renders state-counts strip prominently (OPEN/INVESTIGATING/RESOLVED tallies), hides signal payloads
  - Post-mortem → filter to RESOLVED, show "Write RCA" CTA per row, link to /postmortem

### A5. `SoundAlerts`
- Hook `useP0Beep(items, muted)`. Track `prevP0Ids` in a ref; on new P0 id added to `items` while `muted === false`, play a short tone.
- Use Web Audio API (no asset to bundle): `AudioContext`, 880Hz square wave, 100ms decay.

### A6. `HealthStrip`
- Polls `getHealth()` every 5s. Renders a row: `[ Postgres · 4ms ] [ Mongo · 6ms ] [ Redis · 1ms ] [ Timescale · 5ms ]  |  queue depth bar`.
- Status dot per dep: green if `ok`, amber if degraded, red if down. Latency mono.
- Hidden during the very first poll (skeleton placeholder).

### A7. `QueueGauge`
- Horizontal bar 200×6px. Width proportional to `queue_depth / queue_capacity`.
- ≤60% lime, 60–85% amber, >85% red with `animate-pulse-live`.
- Right-aligned `XXXX/50000` tabular nums.

### B1. `SignalFrequency`
- Input: `signals: Signal[]` (we fetch the first 1–3 pages to get up to ~150 timestamps).
- Bucket `timestamp` values into 12 equally-sized buckets between min and max ts.
- Render as recharts bar chart, height 80px, lime bars.
- Label x-axis with "T0 ... TN-1 (Xm span)" — relative.

### B2. `PayloadFingerprints`
- Input: `signals: Signal[]`.
- Fingerprint = sorted top-level keys of `payload` joined by `|`. (Pure, deterministic, no hashing required — short enough.)
- Group, count, sort desc by count.
- Render as a stack of chips: `[ {error, host} · 187× ] [ {error, host, retry_count} · 8× ] [ 5 other ]`.
- Click a chip → filters the raw signals list below to that fingerprint (set state up the tree).

### B3. `IncidentSeverityOverTime`
- For incidents where signals span >30s, plot severity distribution along time as a stacked bar (using recharts).
- For shorter incidents, render a notice "Burst incident — Xms span" instead.

### B4. `TransitionTimeline`
- Input: `transitions: StateTransition[]` + `work_item` (for `first_signal_ts` and `closed_at` or `last_signal_ts` as right edge).
- Horizontal track. Each transition is a 6×16px tick on the track positioned proportional to `(created_at - start) / (end - start)`.
- Hover a tick: tooltip with `from → to · actor · reason · time`.
- State labels (OPEN, INVESTIGATING, RESOLVED, CLOSED) rendered as colored regions between ticks.

### B5. `TimeInState`
- Compute per-state durations from `transitions[]` and the incident's `first_signal_ts`/`closed_at`/`now()`.
- Render as four stacked horizontal bars (one per state) with `Xm Ys` label + percentage.
- Lime for the currently-active state.

### C1. `MTTRHistogram`
- Input: `items: WorkItem[]` (closed).
- Buckets: `<60`, `60–300`, `300–900`, `900–3600`, `≥3600` seconds.
- Render as a row of 5 vertical bars; tallest = 100% height; rest scale proportionally.
- Lime bars; hover shows count + range.

### C2. `CategoryBreakdown`
- Input: `items: WorkItem[]` (closed) + a function or component that can fetch each item's RCA.
- **Constraint:** `/v1/incidents/closed` doesn't include RCA. To avoid N round-trips, we'll show a notice "RCA breakdown requires per-incident fetch" and lazy-load when expanded, OR we accept N parallel fetches with `Promise.all` (cap at 50). Pick the lazy-load route — render a "Compute" button that fetches.
- Render as a horizontal stacked bar across all 7 categories, severity-keyed palette.
- Below the bar: list of categories with counts.

### C3. `MTTRTrend`
- Input: closed items sorted by `closed_at` asc.
- Plot points `(closed_at → mttr_seconds)` as a recharts LineChart.
- y-axis log-scale if max/min ratio > 50 (MTTR varies wildly).
- Lime line, monotone.

### C4. `RepeatOffenders`
- Input: closed items.
- Group by `component_id`. Compute count, avg MTTR.
- Top 5 only, sorted by count desc.
- Render as a list with severity-color count badges.

### D. `/postmortem` page
- Server component? No — we poll. Client.
- Layout: page header + RCA-quality stats strip (% closed with RCA, avg fix length, avg prevention length, RCAs by author) + queue list of RESOLVED items.
- Queue list: card per RESOLVED item, with "Write RCA →" button → `/incidents/{id}/rca`.
- For each RESOLVED card: component_id, severity, resolved_at, oldest first.
- Uses `listIncidents()` filtered client-side to `status === "RESOLVED"`.

### E1. `/load-test` page
- "Send burst" controls: count (input, default 100, max 10000), rps (input, default 100, max 5000).
- Button → fires off `count` POST `/v1/signals` requests, paced at `rps` (use `setTimeout` ticks).
- Show running counters: sent / accepted (202) / rejected (503) / pending. Pulsing lime dots on active.
- After completion: summary card with throughput, error rate, p99 latency (measured client-side from `performance.now()`).
- Terminal-style chrome to match landing's LogTape vocabulary.

### E2. `/simulate` page
- Three pre-canned scenarios:
  1. **RDBMS cascade** — 50 P0 signals to `RDBMS_PRIMARY_01` + 100 P1 signals to `API_CHECKOUT` over 5s
  2. **Cache thrash** — 200 P2 signals to `CACHE_CLUSTER_A` over 10s
  3. **MCP host fail** — 30 P0 signals to `MCP_HOST_INDEXER` + 80 P1 to varying API components
- Each scenario renders as a card. Click → fires the synthetic POSTs, then deep-links to the live feed.
- Shows debounce ratio prediction inline ("≈3 incidents from 200 signals").

### E3. `/flow` page
- An animated diagram tracing one signal end-to-end. Reuses the language of the landing's `HowItWorks`.
- Stage 1 (ingest, 0–500ms): pulse on the HTTP arrow.
- Stage 2 (debounce, 500–1500ms): Redis Lua box lights up; if existing window, signal joins; else new.
- Stage 3 (persist, 1500–2500ms): three parallel writes (Mongo, Postgres, Timescale) animate.
- Stage 4 (alert, 2500–3000ms): Strategy registry picks alerter; arrow fires off to "PagerDuty stub / Slack / Console".
- Auto-loops; pause button. Persona-friendly explanation strip below.

## 6. `lib/api.ts` additions

```ts
export interface Health {
  status: "ok" | "degraded" | "down";
  uptime_seconds: number;
  queue_depth: number;
  queue_capacity: number;
  dependencies: Record<"postgres" | "mongo" | "redis" | "timescale", Dependency>;
}
export interface Dependency {
  status: "ok" | "degraded" | "down";
  latency_ms: number;
}
export function getHealth(): Promise<Health> { … }

// Helper: fetches up to N pages of signals for histogram/fingerprint use.
export async function listSignalsBulk(id: string, maxPages = 3, perPage = 50)
  : Promise<Signal[]> { … }
```

If `getHealth()` fails or the backend isn't running, the dashboard must
**not** crash — degrade to a "backend offline" chip and keep rendering.

## 7. Hover/glow rules (consistency)

| Surface | Glow color | Trigger |
|---|---|---|
| StatCard | sparkColor | hover |
| IncidentRow | severity | hover |
| New section cards | lime (default) | hover |
| RCA submit btn | lime | hover |
| Closed-incident row | severity | hover (already shipped) |
| MTTRHistogram bars | lime | hover |
| NoisyComponents bars | severity of top incident for component | hover |
| Health dep chips | their status color (green/amber/red) | hover |

## 8. Animation rules

- All count-ups: `easeOutQuart`, ~900ms, gated on `useInView` + `useReducedMotion`.
- New live-feed widgets fade-slide-in once per mount (`fade-slide-in` keyframe already in `tailwind.config.ts`).
- Persona switcher transitions are crossfade only — no layout shifts.
- No animation on the **/flow** page when `prefers-reduced-motion` — render the final state immediately.

## 9. Testing strategy

We can't run Go tests from the frontend, but we can run:

1. `pnpm build` — must compile clean (TypeScript, no warnings beyond the recharts SSR width-0 chatter we already accept).
2. After build, **start prod server (`pnpm start`)** and `curl` every route. Expected 200:
   - `/`
   - `/dashboard`
   - `/incidents/closed`
   - `/incidents/<arbitrary uuid>`
   - `/incidents/<uuid>/rca`
   - `/postmortem`
   - `/load-test`
   - `/simulate`
   - `/flow`
3. **Safety checks for empty-state defenses** — every new widget must render gracefully when its source data is `[]` or `undefined` (the backend may be down).
4. **No fatal errors during SSR** — all client-only APIs (`AudioContext`, `localStorage`, `Math.random`, `performance.now`) gated behind `useEffect`.

## 10. Security & resilience

- The frontend is read-only against the backend (with the exception of POST /rca and PATCH /state, which already exist). The new `/load-test` and `/simulate` pages POST signals; they must:
  - Cap the per-burst signal count at 10,000 client-side to prevent the user from accidentally DoS'ing themselves.
  - Apply a max RPS cap (5000) — well under the backend's 10K design throughput.
  - Show a clear "this fires real POSTs to /v1/signals; running creates real Mongo+Postgres rows" warning chip on both pages.
- No auth means no auth bypass risk; nothing to add.
- All user-typed inputs (load-test count, rps) are clamped + `parseInt`'d; never interpolated into URLs.
- `payload` field on signals is always rendered via `JSON.stringify`, never `dangerouslySetInnerHTML`.
- localStorage usage is namespaced (`vellum.persona`) and never reads back as code.

## 11. Out of scope for this pass (do NOT build)

- Anything from Tier 2 / Tier 3 (no new endpoints, no schema changes).
- WebSocket live feed (Tier 3 #30).
- Time-travel scrubber (Tier 3 #31).
- Search across payloads (Tier 3 #32).
- Grafana sidecar (Tier 3 #33).
- Keyboard shortcuts overlay (Tier 3 #35).

If I run out of time, I cut **`/flow` first** (it's the most cosmetic),
then **`/simulate`** (overlap with `/load-test`), then **MTTRTrend** (a
graph among graphs; loss is small).

## 12. Decisions log entries to add

After completion, append to `docs/decisions.md`:

- 2026-05-11 — Phase 5: dashboard tier-1 buildout
- Why we keep StatCards alongside the new SeverityStackedBar
- Why CategoryBreakdown is lazy-loaded (avoid N round-trips)
- Why /load-test caps counts client-side (don't DoS self)
- Why we use the Web Audio API instead of an MP3 asset for the P0 beep
