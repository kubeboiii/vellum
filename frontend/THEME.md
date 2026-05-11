# THEME.md — Frontend Design Spec

> **Aesthetic:** Dense matte-black SRE war room. Linear/Vercel restraint, with multi-color severity coding for incident states. Every pixel earns its place.
>
> **For Claude / coding agents:** This is the single source of truth for the visual design. Don't deviate without an entry in `decisions.md`. When building a new component, the recipes in §6 are the canonical patterns — mirror them.

---

## 1. Design Principles

1. **Matte, not glossy.** No gradients, no glow, no glassmorphism. Solid surfaces. The only allowed "depth" is a 1px hairline border.
2. **Severity is the loudest signal.** Color is reserved for meaning. P0 = red. P1 = orange. P2 = amber. P3 = blue. CLOSED = neutral gray. Brand UI (nav, buttons) uses neutral surfaces only.
3. **Dense, not sparse.** This is an ops tool. SREs are scanning fast. Tighter line-heights, smaller padding, more rows on screen. Aim for a Bloomberg-terminal information density without the visual chaos.
4. **Mono for data, sans for prose.** Numbers, IDs, timestamps, statuses, component names → JetBrains Mono. Page titles, form labels, paragraph text → Inter.
5. **Motion is functional.** Pulse the P0 dot, animate state transitions, fade in new incidents. No decorative motion.

---

## 2. Color System

### 2.1 Base palette (neutrals)

True matte black, not navy-tinted. Five surface levels.

```css
--bg-base:       #000000;   /* page background — pure black */
--bg-surface:    #0A0A0A;   /* cards, panels */
--bg-elevated:   #141414;   /* hover, dropdowns, modals */
--bg-input:      #0F0F0F;   /* form inputs */
--bg-hover:      #1A1A1A;   /* row hover */

--border-subtle: #1F1F1F;   /* default hairline */
--border-strong: #2A2A2A;   /* emphasis, focused inputs */
--border-focus:  #404040;   /* keyboard focus ring */

--text-primary:   #FAFAFA;  /* near-white, never pure white */
--text-secondary: #A1A1AA;  /* labels, meta */
--text-tertiary:  #71717A;  /* hints, placeholders */
--text-disabled:  #52525B;
```

**Rules:**
- Never use pure `#FFFFFF` for text. Pure white on pure black vibrates. Use `#FAFAFA`.
- The four background levels are not arbitrary. Use them in order: `--bg-base` (page) → `--bg-surface` (cards) → `--bg-elevated` (hover/modals) → `--bg-input` (form fields, slightly recessed).

### 2.2 Brand accent (lime)

Single warm accent color, used sparingly. This is what separates the IMS from "just another dark dashboard." Inspired by terminal-classic green but tuned for modern displays — not the harsh `#00FF00` of phosphor CRTs, but a deliberate, restrained electric lime.

```css
--accent:         #BEF264;   /* lime 300 — primary brand */
--accent-bright:  #D9F99D;   /* lime 200 — hover / active */
--accent-dim:     #65A30D;   /* lime 600 — pressed / secondary use */
--accent-bg:      #0A1004;   /* deep lime wash for backgrounds */
--accent-border:  #365314;   /* lime 900 — outlines on dark surfaces */
--accent-glow:    rgba(190, 242, 100, 0.18);  /* focus ring, subtle pulse */
--accent-text:    #1A2E05;   /* lime 950 — only used as text ON lime bg */
```

**Where to use lime:**
- The `IMS` wordmark in the nav
- Active nav item (left border accent or background tint with `--accent-bg`)
- Primary button background (`Submit & Close`, `Mark as Investigating`)
- Focus rings on inputs and buttons (2px outline, color `--accent`)
- Sparkline strokes on neutral metric cards (e.g. `INGEST RATE` card)
- The "live" indicator dot next to the page title (pulsing slowly)
- Section header underline accents (1px, 24px wide, under section titles)

**Where NOT to use lime:**
- Anywhere severity is being communicated — that's red/orange/amber/blue's job
- For text on dark surfaces (lime on `--bg-base` is too high-contrast for body text; use only for accent labels ≤13px)
- In hover states of incident rows (those get `--bg-hover`, not lime)
- For chart data series (charts use severity colors or neutral grays)

**The text rule:** Lime on dark backgrounds is fine for accents and small labels. Lime as a fill color (button background, badge) needs `--accent-text` (very dark lime) for text on top — never white, never gray. This is the same rule as severity badges.

### 2.3 Severity colors

These encode meaning and **must not be used for decoration elsewhere**.

```css
--sev-p0:        #EF4444;   /* red 500 — critical */
--sev-p0-bg:     #1A0606;   /* deep red wash for backgrounds */
--sev-p0-border: #7F1D1D;   /* red 900 for borders */
--sev-p0-glow:   rgba(239, 68, 68, 0.15);  /* pulse animation */

--sev-p1:        #F97316;   /* orange 500 — high */
--sev-p1-bg:     #1A0F06;
--sev-p1-border: #7C2D12;

--sev-p2:        #F59E0B;   /* amber 500 — medium */
--sev-p2-bg:     #1A1206;
--sev-p2-border: #78350F;

--sev-p3:        #3B82F6;   /* blue 500 — low */
--sev-p3-bg:     #06101A;
--sev-p3-border: #1E3A8A;

--state-open:           #EF4444;   /* same as P0 — incident is live */
--state-investigating:  #F59E0B;   /* amber */
--state-resolved:       #10B981;   /* emerald — fixed but not closed */
--state-closed:         #71717A;   /* gray — done */
```

### 2.4 Status colors (non-severity)

For non-incident things: form validation, health checks, success messages.

```css
--success:  #10B981;   /* emerald */
--warning:  #F59E0B;   /* amber */
--danger:   #EF4444;   /* red */
--info:     #3B82F6;   /* blue */
```

---

## 3. Typography

### 3.1 Fonts

```css
--font-sans: 'Inter', system-ui, -apple-system, sans-serif;
--font-mono: 'JetBrains Mono', 'SF Mono', 'Menlo', monospace;
```

Load via `next/font/google` in `app/layout.tsx`. Set `display: 'swap'`.

### 3.2 Scale (dense)

| Use | Size | Weight | Line height | Font |
|---|---|---|---|---|
| Page title (h1) | 20px | 600 | 1.3 | sans |
| Section title (h2) | 15px | 600 | 1.4 | sans |
| Card title | 13px | 600 | 1.4 | sans |
| Body | 13px | 400 | 1.5 | sans |
| Label | 11px | 500 | 1.4 | sans, uppercase, tracking 0.05em |
| Meta / timestamp | 11px | 400 | 1.4 | mono |
| Data / ID / count | 12px | 400 | 1.4 | mono |
| Stat number (big) | 22px | 500 | 1.2 | mono |
| Code / payload | 12px | 400 | 1.5 | mono |

Note: this is denser than the typical Linear/Vercel scale (which would use 14px body). 13px body is intentional for a dense ops tool.

### 3.3 When to use mono

- All **IDs** (incident_id, signal_id, component_id)
- All **timestamps** (ISO, relative, durations)
- All **numbers** in tables, counters, badges, MTTR
- All **status text** when shown in a table (`OPEN`, `INVESTIGATING`)
- **Code** and signal payload JSON
- The metrics-log line on the homepage hero

Everything else: sans.

---

## 4. Spacing & Layout

Base unit: 4px. Use multiples.

```css
--space-1: 4px;
--space-2: 8px;
--space-3: 12px;
--space-4: 16px;
--space-6: 24px;
--space-8: 32px;
--space-12: 48px;
```

### 4.1 Density rules

- Table rows: **32px height**, 8px vertical padding, 12px horizontal
- Cards: 16px padding (not 24px)
- Section gap: 24px, not 48px
- Form field gap: 16px
- Sidebar width: 220px (narrow, more content room)
- Page max-width: none on the live feed; 720px on RCA form

### 4.2 Border radius

Sharper than typical design systems — fits the SRE aesthetic.

```css
--radius-sm: 4px;     /* badges, pills, small buttons */
--radius-md: 6px;     /* cards, inputs, dropdowns */
--radius-lg: 8px;     /* modals, primary surfaces */
```

No 12px+ radius anywhere. Not a consumer app.

### 4.3 Borders

Always 1px hairlines. Never thicker except for focus ring (2px). No box-shadows for surface separation — use border + bg-level change.

---

## 5. Motion

### 5.1 Tokens

```css
--ease-out: cubic-bezier(0.16, 1, 0.3, 1);   /* Vercel's signature ease */
--ease-in-out: cubic-bezier(0.65, 0, 0.35, 1);

--dur-fast: 120ms;    /* button hover, color change */
--dur-base: 200ms;    /* most transitions */
--dur-slow: 400ms;    /* page sections fading in */
```

### 5.2 What animates

- **Button / link hover:** background-color 120ms ease-out.
- **P0 dot pulse:** infinite, 1500ms. See keyframes in §6.7.
- **New incident appears in live feed:** fade + slide-down, 300ms ease-out. Use Framer Motion.
- **State transition button click:** subtle scale `0.97 → 1` on press, 100ms.
- **Sparkline update:** 200ms ease-out path morph.
- **Modal open/close:** fade + slight scale (0.96 → 1), 200ms.
- **Sound:** a single sine-wave beep at 880Hz, 80ms, on first P0 of a session (and only if the user has interacted with the page — browsers block autoplay otherwise). Mute toggle in the nav.

### 5.3 Reduced motion

Always respect `prefers-reduced-motion: reduce`. Disable pulse, fades, and sound. Animations are nice-to-have, not a feature.

---

## 6. Component Recipes

These are the canonical patterns. Mirror them when building new variants.

### 6.1 Severity badge

```tsx
// components/ui/severity-badge.tsx
// Usage: <SeverityBadge severity="P0" />
const STYLES = {
  P0: 'text-red-400 bg-red-950 border-red-900',
  P1: 'text-orange-400 bg-orange-950 border-orange-900',
  P2: 'text-amber-400 bg-amber-950 border-amber-900',
  P3: 'text-blue-400 bg-blue-950 border-blue-900',
};
// pill: px-2 py-0.5 text-[11px] font-mono font-medium rounded border
// 11px mono uppercase, no letter-spacing change
```

### 6.2 State pill (run-status style)

The state pill is the workhorse of the UI — it appears in every table row, every detail page, every transition control. Pattern borrowed from trigger.dev's run-status pills: tight inline pill with dot + label, monospace, distinct hover.

```tsx
// components/ui/state-pill.tsx
// Anatomy: [ • LABEL ]
//   - 6px dot, severity-keyed color, optionally pulsing
//   - 11px JetBrains Mono, uppercase, tracking 0.04em
//   - background = state's --bg variant at low alpha
//   - 1px border = state's --border variant
//   - radius-sm (4px), px-2 py-0.5
//
// States:
//   OPEN          → red dot,    bg red-950/40,     border red-900,    text red-300
//   INVESTIGATING → amber dot,  bg amber-950/40,   border amber-900,  text amber-300
//   RESOLVED      → emerald,    bg emerald-950/40, border em-900,     text em-300
//   CLOSED        → gray dot,   bg zinc-900/50,    border zinc-800,   text zinc-400
//
// Special: OPEN + P0 → dot pulses (see §6.7). The pill itself does not pulse.
// On hover (when inside a clickable row): no change to the pill itself; the row gets bg-hover.
```

This component appears so often that it deserves attention. Get this one right and the whole UI looks polished.

### 6.3 Incident row (live feed)

Single row in the live feed table:

```
[●] CACHE_CLUSTER_01    P0   OPEN          243 signals    08:42:11   2m ago   →
↑    ↑                  ↑    ↑              ↑              ↑          ↑       ↑
dot  component (mono)   sev  state          count (mono)   time(mono) rel     chevron
```

Layout: `grid-cols-[12px_1fr_50px_120px_120px_100px_90px_20px]`, gap 12px, padding 8px 16px, height 32px. Hover: `bg-elevated`. Click → navigate to detail. Pulse the dot if P0+OPEN.

### 6.4 Stat card (top of dashboard)

```
┌───────────────────────────────┐
│ ACTIVE INCIDENTS              │  ← label, 11px uppercase, secondary
│ 7                          ▁▃▆ │  ← number 22px mono + sparkline
│ +2 from 5m ago                │  ← delta, 11px mono, red if up
└───────────────────────────────┘
```

Surface: `--bg-surface`, 1px border, 16px padding, radius-md. Width 1/4 of grid. Sparkline is 60×16, last 30 data points, stroke-width 1, color matches severity.

### 6.5 Signal payload (incident detail)

JSON shown as syntax-highlighted, mono 12px, line-height 1.5. Keys: text-secondary. Strings: emerald. Numbers: amber. Booleans: blue. Use Prism or a hand-rolled tokenizer; whatever's smallest.

### 6.6 RCA form

Dense vertical form, single column, max-width 720px. Inputs: `--bg-input`, 1px `--border-subtle`, 6px radius, 36px height, 13px text. Focus: border becomes `--border-focus` + 2px ring `--border-focus/0.2`. Labels above inputs, 11px uppercase. Errors below: 11px `--danger`, mono.

### 6.7 The P0 pulse animation

The signature visual moment of the dashboard. Subtle, not aggressive.

```css
@keyframes pulse-p0 {
  0%, 100% { box-shadow: 0 0 0 0 var(--sev-p0-glow); }
  50%      { box-shadow: 0 0 0 6px transparent; }
}
.p0-dot { animation: pulse-p0 1500ms var(--ease-in-out) infinite; }
```

Only applied to `OPEN` P0 incidents. Stops on state change.

### 6.8 Hero signal-rate chart (dash0 style)

The chart at the top of the live feed. Compact but visually significant — when the failure simulator runs, this is what visibly spikes in the demo video.

```
┌─ SIGNAL RATE · last 15 min ─────────────────── 8,421/s ──┐
│                                                          │
│                          ▄▄▄                             │
│                     ▄▄▄▄▄███▄▄▄▄▄                        │
│  ▁▁▂▂▃▃▄▄▄▄▅▅▅▅▆▆▆▆███████████▇▇▆▆▅▅▄▄▃▃▂▂▁▁            │
│ ─────────────────────────────────────────────────────    │
│ 15m ago               7m ago               now           │
└──────────────────────────────────────────────────────────┘
```

**Spec:**
- Full width, 120px tall, sits directly under the page title
- Recharts `AreaChart` with stacked areas by severity (P0 red on top, then P1, P2, P3, then resolved/closed at bottom in gray)
- Areas use severity colors at low alpha (0.15-0.25) for fills, full color for the top stroke line (1px)
- X-axis: rolling 15-minute window, 30-second buckets (30 points). No tick labels except "15m ago", "now"
- Y-axis: hidden. Current rate shown as text in the top-right corner of the card frame
- Grid lines: removed
- On hover: vertical line cursor + tooltip showing exact counts per severity at that bucket
- Background: `--bg-surface`, 1px border, radius-md, padding 12px 16px
- The "current rate" number in the top right is 13px mono, with a small live indicator dot in lime (`--accent`) that pulses very slowly (3s cycle) to indicate the chart is live

**Why this works:** the chart gives the live feed page immediate visual weight. The bumps and spikes tell a story at a glance — flat = healthy, climbing = something's wrong, spike = check the table below. This is the dash0 information-design principle without their gradient treatment.

### 6.9 Primary button (lime)

```tsx
// components/ui/button-primary.tsx
// The high-conviction button: Submit & Close, Confirm State Change, Acknowledge
// - bg: --accent (lime 300)
// - text: --accent-text (very dark lime — NOT white)
// - border: none
// - radius-sm (4px), px-3 py-1.5
// - text: 13px sans, weight 500 (NOT 600 — lime is already loud)
// - hover: bg --accent-bright (lime 200)
// - active: bg --accent-dim (lime 600) + scale 0.98
// - disabled: bg zinc-800, text zinc-600, no cursor
// - focus-visible: 2px ring --accent at 40% alpha, offset 2px from bg-base
```

Use lime sparingly — typically one primary button per page. The "Submit & Close" on the RCA form is the canonical example. Secondary actions use a ghost button (transparent bg, --border-subtle, hover --bg-elevated).

---

## 7. Page Layouts

### 7.1 Live Feed (`/`)

```
┌─────────────────────────────────────────────────────────────────────┐
│ ▎IMS · ● Live Feed                                  🔔 mute    ⚙   │ ← nav 48px
├─────────────────────────────────────────────────────────────────────┤  ▎ = lime accent bar
│ ┌─ SIGNAL RATE · last 15 min ────────────── 8,421/s ●live ─────┐   │ ← hero chart 120px
│ │                       ▄▄▄                                   │   │   ●live = lime dot
│ │                  ▄▄▄▄▄███▄▄▄▄▄                              │   │
│ │ ▁▁▂▂▃▃▄▄▄▅▅▅▅▆▆▆▆█████████████▇▇▆▆▅▅▄▄▃▂▁                   │   │
│ │ 15m ago             7m ago                now                │   │
│ └──────────────────────────────────────────────────────────────┘   │
│                                                                     │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐                │
│ │ ACTIVE   │ │ P0       │ │ AVG MTTR │ │ INGEST   │                │ ← stat cards (compact)
│ │ 7    ▁▃▆ │ │ 2    ▁▁▃ │ │ 14m  ▆▃▁ │ │ 8.4K ▃▆█ │                │   sparklines lime
│ └──────────┘ └──────────┘ └──────────┘ └──────────┘                │
│                                                                     │
│ FILTERS                                                             │
│ [All]  [P0]  [P1]  [P2]  [P3]      State: [Active ▾]                │
│                                                                     │
│ ACTIVE INCIDENTS                                              ────  │ ← section header
│ ● CACHE_CLUSTER_01      [ • OPEN ]          P0   243  08:42  2m  › │   lime underline
│ ○ RDBMS_PRIMARY_01      [ • INVESTIGATING ] P0    89  08:38  6m  › │
│ ○ API_GATEWAY_03        [ • INVESTIGATING ] P1    34  08:30 14m  › │
│ ○ QUEUE_NOTIFICATIONS   [ • OPEN ]          P2    18  08:28 16m  › │
│ ○ MCP_HOST_05           [ • RESOLVED ]      P3     7  08:21 23m  › │
│                                                                     │
│ ── [metrics] accepted=8421/s processed=8398/s queue=312/50000 ──   │ ← bottom strip 11px mono
└─────────────────────────────────────────────────────────────────────┘
```

### 7.2 Incident Detail (`/incidents/[id]`)

Two-column. Left (3/5): metadata, state timeline, transition buttons. Right (2/5): scrollable signal list, click any signal to expand JSON payload inline.

### 7.3 RCA Form (`/incidents/[id]/rca`)

Single column, 720px max. Top: incident summary card (read-only). Below: form fields stacked. Bottom right: `Save Draft` (ghost) + `Submit & Close` (primary, only enabled when all required fields valid).

---

## 8. Implementation Notes for Claude

### 8.1 Stack

- **Next.js 14 App Router** + **TypeScript**
- **Tailwind CSS** with the tokens above mapped into `tailwind.config.ts`
- **shadcn/ui** components copied into `components/ui/` — customize their default classes to match this theme
- **Framer Motion** for the new-incident slide-in and modal animations
- **Recharts** for sparklines and any future charts
- **Tabler Icons** (`@tabler/icons-react`) — outline only, never filled. 16px inline, 20px for nav icons.

### 8.2 Tailwind config tokens

Map the CSS variables into `theme.extend.colors` so you can write `bg-bg-surface`, `text-sev-p0`, etc. Don't hardcode hex anywhere in components.

### 8.3 Dark mode

There is no light mode. The site is dark-only. Don't add a theme toggle.

### 8.4 Forbidden

- **No gradients anywhere.** Not on buttons, not on backgrounds, not on cards.
- **No glassmorphism / backdrop-blur.** Solid surfaces only.
- **No emoji in UI** (Tabler icons only).
- **No drop-shadows for elevation.** Use border + bg-level change.
- **No rounded-full on anything except dots and avatars.** Pills use `radius-sm`.
- **No font weights other than 400, 500, 600.** No 300 thin, no 700 bold.
- **Don't render incident IDs as long UUIDs.** Truncate to first 8 chars with `font-mono`.

### 8.5 Accessibility despite dense layout

- All interactive elements need a visible focus ring (2px `--border-focus`).
- Color is never the only signal — every severity badge has a letter (P0, P1) too.
- Hit targets minimum 32×32px.
- `prefers-reduced-motion` disables pulse + slide-ins.
- Sound is opt-out (mute button in nav), and never autoplays before user interaction.

---

## 9. Inspiration anchors (for Claude's reference)

The visual language is the love-child of four reference sites. When Claude is unsure how something should look, check what these would do.

| Site | What we take from it |
|---|---|
| **trigger.dev** | The run-status pill pattern (dot + label, tight mono badge with border). Status-as-visual-anchor in tables. Confident dev-tool density. |
| **warp.dev** | Sharp corners (4-8px max, never 12px+). Mono-first typography in unexpected places. Crisp solid-fill selection/active states — no washes, no fades. Restrained accent usage. |
| **dash0.com** | Chart-first dashboard layout — the hero signal-rate chart at the top of the live feed is directly inspired by their information design. Live data as the primary visual element. **Note:** dash0 uses subtle gradients on hero text; we explicitly do not. We take their info design, not their gradient treatment. |
| **blocks.shawnlukas.com** | Typographic restraint and detail. Aggressive monospace usage for labels (not just data). Hairline 1px dividers between sections. The general "every detail considered" feel. |

Secondary anchors (these influenced earlier decisions):
- **Linear** — table row density, the cubic-bezier ease, weight discipline (no 700 bold)
- **Vercel dashboard** — the specific matte black surface levels (`#000` → `#0A0A0A` → `#141414`)
- **Datadog incident view** — severity color coding, the way state pills appear in table rows
- **Sentry issue list** — JSON payload display, stack-trace style information density

**If Claude generates something that looks like Stripe, Notion, or any consumer SaaS app** (lighter, rounder, friendlier, more whitespace), it's wrong for this project. The IMS is an ops tool, not a marketing site.

The single brand accent — lime — is the IMS's signature. Used sparingly: wordmark, active nav, primary buttons, focus rings, live-indicator dots, sparkline strokes on neutral cards. Never on severity-encoded UI. The combination of pure matte-black surfaces + multi-color severity coding + a single restrained lime accent is what makes this distinct from the four references rather than a copy of any one of them.

---

## 10. What "done" looks like for a component

When Claude finishes a component, check:

- [ ] Uses only tokens from this file (no raw hex, no arbitrary spacing values)
- [ ] Font-family is explicit (sans for prose, mono for data)
- [ ] All interactive states implemented: default, hover, focus, active, disabled
- [ ] `prefers-reduced-motion` respected if it animates
- [ ] No emoji, no gradients, no shadows
- [ ] Densest reasonable padding (when in doubt, tighter not looser)
- [ ] Severity colors used only for severity, never decoration
- [ ] Renders correctly with 0 data, 1 row, 10000 rows
- [ ] Keyboard navigable (tab, enter, escape on modals)
