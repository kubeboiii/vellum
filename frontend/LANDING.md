# LANDING.md — Landing Page Design Spec

> **Companion to:** `frontend/THEME.md` — read that first. This document does not redefine the color system, typography, or motion. It extends them with landing-page-specific patterns: marketing sections, hero, feature blocks, code tabs, trust strips, and a few new tokens (violet annotation accent, hand-drawn arrows).
>
> **Scope:** the root route `/` (the landing page). The dashboard at `/dashboard` and all incident pages stay governed by `THEME.md`.
>
> **For Claude:** the rules from `THEME.md` (no gradients, no shadows, no glassmorphism, no emoji, dark-only, 13px body, JetBrains Mono for data) still apply. The landing page is denser than typical marketing sites and shares the dashboard's matte-black surfaces. The only thing that changes is layout patterns and one new accent color (violet, for handwritten annotations).

---

## 1. Why the landing page exists

It exists because reviewers, recruiters, and hiring managers will land at `/` first when they open your project. They will spend ~30 seconds before deciding whether to dig into the code. The landing page is the elevator pitch.

Goals, in order:

1. **In 5 seconds**: tell the visitor what the Vellum is and that it's serious.
2. **In 30 seconds**: convince them the engineering inside is good enough to investigate.
3. **In 2 minutes**: get them to click through to `/dashboard` and see the product live.

Non-goals: collecting emails, selling anything, A/B testing. There are no forms.

---

## 2. The single difference from `THEME.md`

The dashboard is dense, scannable, and emotionally neutral. The landing page is **spacious, narrative, and emotionally confident**. Same colors, same fonts, different rhythm.

| | Dashboard | Landing |
|---|---|---|
| Section padding (vertical) | 24px | 96px–128px |
| Max content width | none (full bleed) | 1120px, centered |
| Body font size | 13px | 15–16px |
| Headline scale | 20px max | up to 64px |
| Line length | dense | comfortable (60–75ch) |
| Negative space | minimal | generous |
| Tone | "scan this fast" | "consider this carefully" |

Everything else — colors, mono usage, no-gradient rule, severity coloring, lime accent — is identical.

---

## 3. New tokens (extends `THEME.md` §2)

### 3.1 Annotation violet

Inspired by Qovery's handwritten loop. Used **only** for the italic-script annotation accents and their hand-drawn SVG arrows. Never for body UI, never for severity, never for anything else on the page.

```css
--annotation:        #A78BFA;   /* violet 400 — italic script + arrows */
--annotation-dim:    #7C3AED;   /* violet 600 — pressed state on annotation links */
```

**Strict usage:**
- Exactly **two** annotations per page. No more.
- Each annotation is `~14px`, italic, font: `var(--font-serif)` (we'll add a serif — see §4).
- Each annotation pairs with a hand-drawn SVG arrow pointing to the thing being called out.
- Annotations are decorative, not informational. They emphasize something already explained in the body text.

### 3.2 Serif font (new)

The italic annotations need a serif. Use **Instrument Serif** from Google Fonts — same as Vercel, Linear marketing, and many modern dev-tool sites. Italic-only weight loaded.

```ts
// app/layout.tsx
import { Instrument_Serif } from 'next/font/google';
const serif = Instrument_Serif({
  weight: '400',
  style: 'italic',
  subsets: ['latin'],
  variable: '--font-serif',
  display: 'swap',
});
```

```css
--font-serif: 'Instrument Serif', Georgia, serif;
```

The serif is reserved for two contexts on the landing page:
1. Annotation accents (described above)
2. **One** decorative italic word inside the main hero headline (one word maximum — see §6.1)

That's it. No serif body text. No serif subheadings. The serif is a *deliberate moment*, not a font family.

### 3.3 Section dividers

Hairlines between major sections, going edge-to-edge across the viewport.

```css
--divider: rgba(255, 255, 255, 0.06);   /* slightly stronger than border-subtle */
```

Use as `border-top: 1px solid var(--divider)` on each section component.

---

## 4. Page structure

The landing page is one continuous scroll, organized as nine sections in this exact order:

1. **Nav** — fixed, transparent on scroll-top, becomes `--bg-surface` with hairline border on scroll-down
2. **Hero** — headline, subhead, two CTAs, ambient terminal-style background
3. **The problem strip** — short bridge: "production breaks. signals drown you. you need a way to triage."
4. **Before / after comparison** — paired dark cards, problem state vs Vellum state (image 2 pattern)
5. **How it works** — vertical flow with mono section labels (image 3 pattern)
6. **Architecture pattern cards** — three cards with mini-diagrams: ingestion, debounce, workflow (image 7 pattern)
7. **Live code tabs** — interactive tabs showing real code snippets from the actual repo (image 5 pattern)
8. **Capabilities grid** — six small feature tiles with metric numbers
9. **Trust + closing** — tech stack logos strip + final CTA to dashboard (image 4 + 9 pattern)
10. **Footer** — minimal, three columns: project, links, "built by"

Total scroll: about 6–8 viewport heights at 1440×900. Don't make it taller — every section earns its place.

---

## 5. Section recipes

These are the canonical patterns. Each section maps to a numbered subsection here.

### 5.1 Nav

```
┌─────────────────────────────────────────────────────────────────────┐
│ ▎Vellum   PRODUCT   ARCHITECTURE   GITHUB ↗                  Open ›   │
└─────────────────────────────────────────────────────────────────────┘
```

Spec:
- Height **56px**
- Logo: `▎Vellum` — lime vertical bar (3px wide, 18px tall) + wordmark in 14px sans 500
- Center links: 13px mono uppercase, `--text-secondary`, hover → `--text-primary`, no underline
- Right CTA: ghost button `Open ›` linking to `/dashboard`. Ghost = transparent bg, 1px `--border-subtle`, hover `--bg-elevated`
- Default state: transparent background, no border. On scroll > 24px: background becomes `--bg-surface`, bottom border `--divider`, 200ms ease-out transition
- Fixed position, `z-index: 50`

### 5.2 Hero

The most important section. Pattern: short pre-headline pill, large headline with one italic serif word, single-paragraph subhead, two CTAs, ambient log-tape background.

```
┌──────────────────────────────────────────────────────────────────┐
│                                                                  │
│        ┌─────────────────────────────┐                           │
│        │ ● PRODUCTION-GRADE INCIDENT │                           │ ← pre-headline pill
│        │   MANAGEMENT                │   mono 11px               │
│        └─────────────────────────────┘                           │
│                                                                  │
│        Ten thousand signals a second.                            │ ← H1 line 1, sans
│        One incident at a time.                                   │ ← H1 line 2, with
│                                                       ╱ italic ╲ │   italic serif emphasis
│                                            "calmly,"             │ ← optional italic word
│                                                                  │
│        A high-throughput ingestion pipeline,                     │ ← subhead 16px sans
│        Redis-atomic debouncer, and stateful                      │
│        incident workflow — built in Go, with                     │
│        a war-room dashboard in Next.js.                          │
│                                                                  │
│        [ Open dashboard › ]   [ View on GitHub ↗ ]               │ ← primary lime + ghost
│                                                                  │
│        ╱─── annotation arrow                                     │
│       (    italic violet: "live demo — try the simulator"        │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ 08:42:11 ▎ accepted=8421/s processed=8398/s queue=312    │   │ ← ambient log tape
│  │ 08:42:16 ▎ accepted=8714/s processed=8702/s queue=287    │   │   bottom of viewport
│  │ 08:42:21 ▎ accepted=9013/s processed=8992/s queue=341    │   │   mono 11px text-tertiary
│  └──────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
```

**Hero spec:**

- Min-height `calc(100vh - 56px)`, centered vertically (`display: grid; place-content: center`)
- Max-width `880px`, centered horizontally
- **Pre-headline pill**: mono 11px uppercase, lime dot (6px) + label, padding `4px 12px`, border 1px `--accent-border`, radius-sm, bg `--accent-bg`. Centered above headline.
- **Headline**: 56–64px sans, weight 500 (not 600 — at this size 500 reads bold enough), line-height 1.05, letter-spacing `-0.02em`, color `--text-primary`. Two lines. One word in `var(--font-serif)` italic if it fits the rhythm — common pattern is the second line's last word ("calmly", "fast", "live") but use sparingly.
- **Subhead**: 16–18px sans, weight 400, line-height 1.5, max-width 60ch, color `--text-secondary`. Two to four lines.
- **CTAs**: row, gap 12px, 32px below subhead.
  - Primary: lime button per `THEME.md` §6.9, slightly larger here (`px-5 py-3`, text 14px)
  - Secondary: ghost button, same size, text-secondary, hover `--bg-elevated`
- **Annotation** (the Qovery move): one hand-drawn SVG arrow pointing from a position below the CTAs, with `--font-serif` italic violet text alongside it. Approximate text: *"live demo — try the simulator"* pointing to the primary CTA. See §7 for arrow recipe.
- **Ambient log tape**: pinned to bottom of hero (still inside the section, with 32px gap above it). Three rows of mock metrics-line output from your real backend, monospaced 11px `--text-tertiary`, opacity 0.5. Animate by replacing the top row every 5 seconds with a new realistic line and pushing others down — **uses `requestAnimationFrame` and respects `prefers-reduced-motion`** (if reduced, show three static rows).
- **No background image, no gradient mesh, no particles.** The log tape is the only ambient element.

### 5.3 Problem strip

A bridge between hero and the comparison cards. Single paragraph, large, restrained.

Spec:
- Section padding 96px vertical
- Max-width 720px centered
- Single `<p>` element, 22–24px sans regular, line-height 1.45, color `--text-primary` for the main idea, `--text-secondary` for connectives
- Example copy structure (Claude, write actual copy in the same shape):
  > **Production breaks.** Errors arrive by the thousand from APIs, caches, queues, databases. **You can't read them. You can't sort them. You can't even keep up.** The Vellum turns that flood into a small, structured list of incidents your team can actually work.
- **Bold** spans use weight 500 (not 600).
- No card, no border, no surface — sits naked on the page background.

### 5.4 Before / after comparison cards

The Qovery before/after pattern. Two cards side-by-side at desktop, stacked on mobile. Each card has a mono uppercase label with a colored dot, an internal diagram, and a list of consequences/benefits below.

```
┌──────────────────────────────────┐  ┌──────────────────────────────────┐
│ ● WITHOUT AN Vellum                 │  │ ● WITH THE Vellum                   │  ← mono labels
│                                  │  │                                  │     left: red dot
│   [SRE] ←──── [API errors]       │  │   [Signals]                      │     right: lime dot
│      ←─────── [Cache failures]   │  │       │                          │
│      ←─────── [DB timeouts]      │  │       ▼                          │
│      ←─────── [Queue backlogs]   │  │   [DEBOUNCE]                     │
│      ←─────── [...]              │  │       │                          │
│                                  │  │       ▼                          │
│                                  │  │   [Work items] ──▶ [SRE]         │
│                                  │  │                                  │
│ ✕ 10,000+ signals/sec            │  │ ✓ 100:1 noise reduction          │
│ ✕ No correlation                 │  │ ✓ Single source of truth         │
│ ✕ No state, no audit             │  │ ✓ Mandatory RCA + auto MTTR      │
└──────────────────────────────────┘  └──────────────────────────────────┘
```

**Spec:**

- Two cards in a 2-column grid, `gap: 24px`
- Each card: `--bg-surface`, 1px `--border-subtle`, radius-lg, padding `32px`, min-height 420px
- Card label at top: 11px mono uppercase, with a 6px dot (left: `--sev-p0` red, right: `--accent` lime), tracking 0.05em
- Diagram inside the card: SVG, ~280px tall, drawn with mono labels per **§8 SVG diagram recipe**
- Below diagram: 32px gap, then a vertical list of 3 outcomes
  - Left card: each item starts with `✕` (tabler `ti-x`) in `--sev-p0`, then mono 12px label
  - Right card: each item starts with `✓` (tabler `ti-check`) in `--accent`, then mono 12px label
  - Item gap 12px, no border between
- The contrast in dot color (red vs lime) tells the entire story before the user reads anything

### 5.5 How it works (vertical flow)

The Qovery vertical-flow pattern. Three labeled regions stacked vertically, with arrows between them. The middle region (the active product) gets a soft tint.

```
┌─────────────────────────────────────────────────────────────────┐
│ SIGNALS  What flows into the system                             │ ← mono uppercase
│                                                                  │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌──────────┐  │
│   │ APIs       │  │ Caches     │  │ Queues     │  │ Databases│  │ ← source cards
│   └────────────┘  └────────────┘  └────────────┘  └──────────┘  │
└─────────────────────────────────────────────────────────────────┘
                              ↓                                      ← lime arrow
┌─────────────────────────────────────────────────────────────────┐
│ ▎ Vellum  Every signal is debounced, every incident is tracked     │ ← lime bg tint
│                                                                  │     accent-bg
│                  ┌──────────────────┐                            │
│                  │  Ingestion API   │                            │
│                  └──────────────────┘                            │
│                          ↓                                       │
│                  ┌──────────────────┐                            │
│                  │  Debouncer       │                            │
│                  └──────────────────┘                            │
│                          ↓                                       │
│                  ┌──────────────────┐                            │
│                  │  Workflow + RCA  │                            │
│                  └──────────────────┘                            │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│ OUTPUT  What humans and downstream systems see                  │
│                                                                  │
│   ┌────────────┐  ┌────────────┐  ┌──────────────┐              │
│   │ Dashboard  │  │ Alerts     │  │ Audit trail  │              │
│   └────────────┘  └────────────┘  └──────────────┘              │
└─────────────────────────────────────────────────────────────────┘
```

**Spec:**

- Each region is a full-width container, `padding: 48px 32px`
- **Top and bottom regions**: bg `--bg-surface`, 1px `--border-subtle`, radius-lg, color `--text-secondary` for the label
- **Middle region**: bg `--accent-bg` (deep lime wash), 1px `--accent-border`, radius-lg, label includes the lime `▎` accent bar prefix and text in `--accent` for "Vellum" (just that word), rest of label text in `--text-secondary`
- Region label: 11px mono uppercase tracking 0.05em + a 14px sans regular sentence after it, separated by 16px
- Inside top/bottom regions: row of small cards in a 4-column grid, `gap: 12px`. Each card 1px `--border-subtle`, radius-md, padding `12px 16px`, label centered, 13px mono. Icons from Tabler 16px, color `--text-secondary`, to the left of the label.
- Inside middle region: vertical stack of 3 nodes centered, max-width 320px, each node a card identical to the side cards above but a bit larger (`padding: 16px 24px`)
- Arrows between regions: 24px tall, centered, drawn with SVG. Color `--accent` for the arrow between regions (lime). Use a sharp arrowhead, 1.5px stroke. See §7.
- This is the page's second annotation moment (optional but recommended): an italic violet *"this is the part you built"* pointing to the middle region's label

### 5.6 Architecture pattern cards (Trigger.dev pattern cards)

Three cards in a row, each shows a small architecture diagram with mono labels and colored connectors. Below each diagram: title (sans 18px), description (13px text-secondary), "Read more →" link.

```
┌─────────────────────────┐  ┌─────────────────────────┐  ┌─────────────────────────┐
│                         │  │                         │  │                         │
│   [HTTP]───┐            │  │      [Signal]           │  │   [OPEN]→[INVESTIG]     │
│            ▼            │  │         ▼               │  │       ↓                 │
│        [CHANNEL]        │  │     [REDIS LUA]         │  │    [RESOLVED]→[CLOSED]  │
│        ▎▎▎▎▎ 50K        │  │     atomic check        │  │              ↓          │
│            ▼            │  │         ▼               │  │         requires RCA    │
│       [WORKERS×16]      │  │     [WORK ITEM]         │  │                         │
│                         │  │                         │  │                         │
├─────────────────────────┤  ├─────────────────────────┤  ├─────────────────────────┤
│                         │  │                         │  │                         │
│ Backpressured ingestion │  │ Atomic debouncing       │  │ Stateful workflow       │
│ Bounded channel + worker│  │ One Lua script collapses│  │ State pattern enforces  │
│ pool. When full, return │  │ 100 correlated signals  │  │ no CLOSE without RCA.   │
│ 503 — never block, never│  │ into one work item.     │  │ MTTR computed on close. │
│ crash.                  │  │                         │  │                         │
│                         │  │                         │  │                         │
│ Read more ↗             │  │ Read more ↗             │  │ Read more ↗             │
└─────────────────────────┘  └─────────────────────────┘  └─────────────────────────┘
```

**Spec:**

- Three cards, equal width, `grid-template-columns: repeat(3, 1fr)`, `gap: 16px`. Stack to single column below 900px.
- Each card: `--bg-surface`, 1px `--border-subtle`, radius-lg. Top half: diagram. Bottom half: copy. Total min-height ~440px.
- Diagram region: padding 32px, background `--bg-base` (one shade darker than the card itself to create depth without a shadow), 1px bottom-border `--border-subtle`. SVG sits inside, fills the available space, centered.
- Copy region: padding 24px 24px 20px
  - Title: 18px sans weight 500, `--text-primary`, margin-bottom 8px
  - Description: 13px sans regular, line-height 1.55, `--text-secondary`, three lines max
  - "Read more ↗": 13px mono uppercase tracking 0.04em, `--accent`, hover `--accent-bright`, links to anchor on the same page or to GitHub
- Hover state on whole card: `--border-strong` on outer border, 120ms ease-out, no transform
- Each diagram uses **one** color family for its non-neutral elements — first card uses `--sev-p0` for the channel (signals concept), second uses violet (`--annotation`) for the Lua line (it's the "magic" piece), third uses `--accent` lime for the RCA gating. This is the only place violet appears outside of annotations.

### 5.7 Live code tabs (Trigger.dev tabbed code block)

Interactive tabs above a real code block (left) with a prose explanation (right). The code is **real code** from your actual backend. Syntax highlighted using **Shiki** with the `github-dark-dimmed` theme (modified — see below).

```
┌──────────────────────────────────────────────────────────────────────────┐
│ [● Ingestion]  [○ Debounce]  [○ State machine]  [○ Alerter]              │ ← tabs
├──────────────────────────────────────────────────────────────────────────┤
│ ╭───────────────────────────────────╮  Real code from the repo.         │
│ │ // backend/internal/ingest/http.go│                                    │
│ │                                    │  This handler accepts a signal,  │
│ │ func (s *Server) PostSignal(...) { │  validates it, and pushes it     │
│ │     select {                       │  onto the bounded channel.       │
│ │     case s.queue <- sig:           │                                    │
│ │         return c.JSON(202, ...)    │  When the channel is full, we   │
│ │     default:                       │  return 503 immediately. That's │
│ │         return c.JSON(503, ...)    │  the entire backpressure story.  │
│ │     }                              │                                    │
│ │ }                                  │  View full file ↗                │
│ ╰───────────────────────────────────╯                                    │
└──────────────────────────────────────────────────────────────────────────┘
```

**Spec:**

- Section bg `--bg-base`, two-column layout, `grid-template-columns: 1fr 1fr` with `gap: 24px`
- **Tab bar** above both columns: row of pill tabs, mono 11px uppercase, padding `6px 12px`, gap 8px
  - Inactive tab: `--text-secondary` text, 1px `--border-subtle` border, transparent bg, ○ dot to the left
  - Active tab: `--text-primary` text, 1px `--border-strong` border, `--bg-elevated` bg, ● dot in `--accent` lime to the left
  - Click → switch the code block + prose on the right
- **Code block (left)**: `--bg-surface`, 1px `--border-subtle`, radius-md, padding `20px 24px`. Min-height 320px so tab-switching doesn't reflow.
  - First line is a file-path comment in `--text-tertiary` — e.g. `// backend/internal/ingest/http.go`
  - JetBrains Mono 13px, line-height 1.6
  - Syntax colors: tuned to the matte-black aesthetic. Keywords coral `#F87171`, strings emerald `#34D399`, comments `--text-tertiary`, function names `--accent`, numbers `--sev-p2` amber. Use Shiki with a custom theme JSON; sample tokens below.
- **Prose (right)**: padding `20px 24px`
  - 15px sans regular, line-height 1.55, `--text-secondary`
  - Two short paragraphs, 3–4 sentences each
  - "View full file ↗" link at the bottom: 13px mono uppercase tracking 0.04em, `--accent`, links to GitHub at the exact line range
- Tab content changes instantly (no fade). Tabs are keyboard-navigable (arrow keys, enter to activate).
- This is the strongest credibility-building section. Get it right.

**Custom Shiki theme tokens** (drop into a `themes/vellum-dark.json` and pass to Shiki):

```json
{
  "name": "vellum-dark",
  "type": "dark",
  "colors": { "editor.background": "#0A0A0A", "editor.foreground": "#FAFAFA" },
  "tokenColors": [
    { "scope": ["comment"], "settings": { "foreground": "#71717A", "fontStyle": "italic" } },
    { "scope": ["keyword", "storage.type"], "settings": { "foreground": "#F87171" } },
    { "scope": ["string", "string.quoted"], "settings": { "foreground": "#34D399" } },
    { "scope": ["constant.numeric"], "settings": { "foreground": "#F59E0B" } },
    { "scope": ["entity.name.function", "support.function"], "settings": { "foreground": "#BEF264" } },
    { "scope": ["variable", "variable.other"], "settings": { "foreground": "#FAFAFA" } },
    { "scope": ["entity.name.type", "support.type"], "settings": { "foreground": "#A78BFA" } }
  ]
}
```

### 5.8 Capabilities grid

Six small feature tiles in a 3×2 grid (3-column desktop, 2-column tablet, 1-column mobile). Each tile is a single metric or capability stated with a number.

```
┌────────────────────────┐ ┌────────────────────────┐ ┌────────────────────────┐
│ 10K /sec               │ │ 100 → 1                │ │ < 50ms                 │ ← stat 32px mono
│ INGESTION              │ │ DEBOUNCE RATIO         │ │ p99 LATENCY            │ ← 11px mono caps
│                        │ │                        │ │                        │
│ Bounded channel + go   │ │ One Redis Lua script   │ │ Non-blocking handler.  │ ← 13px text-secondary
│ workers absorb bursts. │ │ collapses correlated   │ │ Persistence runs       │
│                        │ │ signals per component. │ │ behind a queue.        │
└────────────────────────┘ └────────────────────────┘ └────────────────────────┘
┌────────────────────────┐ ┌────────────────────────┐ ┌────────────────────────┐
│ 4 stores               │ │ 0 dropped              │ │ 100%                   │
│ POLYGLOT PERSISTENCE   │ │ DEAD-LETTER RECOVERY   │ │ RCA COVERAGE           │
│ ...                    │ │ ...                    │ │ ...                    │
└────────────────────────┘ └────────────────────────┘ └────────────────────────┘
```

**Spec:**

- 3-column grid, `gap: 1px`, `bg: --border-subtle` — the hairline gaps are the borders. Each tile is `--bg-surface` so the grid background "shows through" as 1px lines.
- Tile padding `28px 24px`, min-height 160px
- **Big number**: 32px JetBrains Mono weight 500, `--text-primary`, line-height 1.1, letter-spacing `-0.01em`. Includes its unit (e.g. `10K /sec`, `< 50ms`, `100 → 1`).
- **Label**: 11px mono uppercase tracking 0.05em, `--text-secondary`, margin-top 8px
- **Description**: 13px sans regular, `--text-tertiary`, line-height 1.55, margin-top 16px, max two lines
- No hover state — these are read, not clicked

### 5.9 Trust + closing

Final section before the footer. Two parts: a tech-stack logo strip (Qovery distros pattern) and a closing card with the final CTA.

**Tech stack strip:**

```
┌──────────────────────────────────────────────────────────────────────┐
│        BUILT WITH                                                    │ ← 11px mono caps centered
│                                                                      │
│  [Go]  [Postgres]  [MongoDB]  [Redis]  [Timescale]  [Next.js]        │ ← logo row
└──────────────────────────────────────────────────────────────────────┘
```

Spec:
- Section padding 96px top, 64px bottom
- Label: 11px mono uppercase tracking 0.05em, `--text-secondary`, centered, margin-bottom 32px
- Logos: row of monochrome (single-color) SVG logos at 32px height, gap 48px, centered, `opacity: 0.55`, hover `opacity: 1` with 200ms transition. Use the official wordmarks recolored to `--text-primary`. No vendor colors — these are not endorsements, they're a "built with" statement.

**Closing card:**

Below the strip, a single centered card with the final pitch and CTA:

```
┌───────────────────────────────────────────────────────────────┐
│                                                               │
│                  Stop reading errors.                         │ ← 36px sans
│                  Start resolving incidents.                   │
│                                                               │
│                                                               │
│        [ Open the dashboard › ]                               │ ← lime primary
│                                                               │
│        Or read the engineering writeup ↗                      │ ← ghost text link
│                                                               │
└───────────────────────────────────────────────────────────────┘
```

Spec:
- Max-width 720px, centered, padding `96px 32px`
- `--bg-surface`, 1px `--border-subtle`, radius-lg
- Headline: 36px sans weight 500, `--text-primary`, line-height 1.2, two lines, centered
- Primary CTA: lime button, slightly larger (`px-6 py-3`, 14px text), margin-top 40px
- Secondary text link below: 13px sans, `--text-secondary`, hover `--text-primary`

### 5.10 Footer

Three columns, minimal, sits below the closing card on `--bg-base`.

```
┌──────────────────────────────────────────────────────────────────────┐
│                                                                      │
│   ▎Vellum                  PROJECT              LINKS                   │
│   Incident Management   README                GitHub ↗               │
│   System.                Architecture         Demo video ↗           │
│                          Decisions log        License                │
│                                                                      │
│   ─────────────────────────────────────────────────────────────────  │
│   Built solo · 2026                                       v1.0.0      │ ← 11px mono caps
└──────────────────────────────────────────────────────────────────────┘
```

Spec:
- Padding `64px 32px 32px`, `--bg-base`, 1px top-border `--divider`
- Three columns at desktop, single column on mobile
- Column headers: 11px mono uppercase tracking 0.05em, `--text-secondary`
- Column links: 13px sans, `--text-secondary`, hover `--text-primary`, no underline default, underline on hover
- Bottom row: hairline `--divider` above, mono 11px `--text-tertiary`, left-right split

---

## 6. Annotation accent recipes (the Qovery move)

This is the signature visual flourish of the landing page. **Two annotations per page, no more.** One in the hero, one in the "how it works" section.

### 6.1 Anatomy

An annotation is two things composited:

1. **Hand-drawn SVG arrow** in `--annotation` violet
2. **Italic serif label** in `--annotation` violet, 14–18px, `var(--font-serif)`

```
                                ╱─────╮
                               (       ╲    "try the simulator"
                                ╲       ▼
                                ─────────
                                    ▼
                               [primary CTA]
```

### 6.2 Arrow recipe (SVG)

Hand-drawn means **slightly imperfect**. Use a quadratic or cubic Bézier curve, never a straight line. 1.5–2px stroke, `stroke-linecap: round`, `stroke-linejoin: round`.

```tsx
// components/landing/hand-arrow.tsx
// Usage: <HandArrow variant="loop-down" />
//
// Three variants are enough for the whole page:
//   loop-down  → for hero, curves from label down to CTA
//   curl-right → for how-it-works, curls from label across to the Vellum box
//   wiggle     → for capabilities grid (optional 3rd annotation, but use sparingly)
//
// Each is a <svg> with viewBox sized to its purpose, a single <path>, and
// an arrowhead marker at the end. Stroke: var(--annotation). No fill.
// The path uses M C C ... commands to create the wobble — not perfectly smooth.
//
// Example arrowhead marker (define once in defs):
//   <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5"
//           markerWidth="6" markerHeight="6" orient="auto-start-reverse">
//     <path d="M0,0 L10,5 L0,10 z" fill="var(--annotation)" />
//   </marker>
```

Two important details:
- The path **deliberately wobbles** — humans don't draw perfect curves. Vary the control points by a few pixels from the "geometric" path.
- The arrowhead is filled, not stroked, and sits *on* the path's terminal point with `orient="auto"`.

### 6.3 Where each annotation goes

**Hero annotation:**
- Position: absolutely positioned 24px below the CTA row, offset left
- Text: *"live demo — click here"* or similar, ending mid-sentence (no period)
- Arrow: variant `loop-down`, curves left-then-up-then-right pointing at the primary CTA
- Approx width 220px, height 80px

**How-it-works annotation:**
- Position: absolutely positioned 16px to the right of the middle "▎ Vellum" region label
- Text: *"this is the part you built"* (you can adjust the copy — the point is to call out the active region)
- Arrow: variant `curl-right`, curls from text down-left into the middle region
- Approx width 240px, height 60px

**Reduced motion:** annotations don't animate. They appear in place on render. No fade-in, no draw-on-scroll. (If you want to be fancy, you *can* draw-on-scroll using `stroke-dasharray` animation, but only if `prefers-reduced-motion` is not set. Default behavior should be static.)

---

## 7. SVG diagram recipe (for comparison cards + pattern cards)

The landing page is dense with small SVG diagrams (in §5.4 and §5.6). They all follow the same visual language so the page feels cohesive.

**Tokens for diagrams:**

```css
--diagram-stroke:   #2A2A2A;   /* base lines, same as --border-strong */
--diagram-label:    #A1A1AA;   /* same as --text-secondary */
--diagram-active:   #BEF264;   /* same as --accent — for "happy path" highlights */
--diagram-problem:  #EF4444;   /* same as --sev-p0 — for "bad path" highlights */
```

**Rules:**

- Boxes/nodes: 1px stroke `--diagram-stroke`, no fill (transparent), radius `var(--radius-md)`. Internal padding `12px 16px`.
- Labels inside boxes: JetBrains Mono 11px, `--text-primary`, uppercase, tracking 0.04em
- Connector lines: 1px stroke `--diagram-stroke`, `stroke-linecap: round`. Use **right-angle paths** (Manhattan routing) not curves — these are technical diagrams, not flowcharts.
- Arrowheads: small triangle, filled with the line color, defined once as a `<marker>` in `<defs>`
- **Highlighted path** (the "happy path" or "this is what you built" path): use `--diagram-active` lime for the stroke and the destination node's stroke. Optionally make the line `stroke-dasharray: none` (solid) while other lines are `stroke-dasharray: 4 4` (dashed) to further emphasize.
- No drop-shadows, no gradients, no rounded curves on connector lines.
- All diagrams export sized to viewBox 100% width, height auto, so they scale to container.

**Example: a minimal 3-node flow in SVG:**

```svg
<svg viewBox="0 0 320 200" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <marker id="arr" viewBox="0 0 10 10" refX="8" refY="5"
            markerWidth="6" markerHeight="6" orient="auto-start-reverse">
      <path d="M0,0 L10,5 L0,10 z" fill="#2A2A2A" />
    </marker>
  </defs>
  <!-- Boxes -->
  <rect x="20" y="20" width="120" height="40" rx="6"
        fill="transparent" stroke="#2A2A2A" stroke-width="1"/>
  <text x="80" y="44" text-anchor="middle"
        font-family="JetBrains Mono" font-size="11"
        fill="#FAFAFA" letter-spacing="0.04em">SIGNAL</text>
  <!-- ...repeat for other nodes... -->
  <!-- Connector -->
  <path d="M140 40 L180 40" stroke="#2A2A2A" stroke-width="1"
        fill="none" marker-end="url(#arr)"/>
</svg>
```

---

## 8. Implementation order

Build in this order — each step is a clean checkpoint:

1. **Tokens & fonts.** Add `--annotation`, `--annotation-dim`, `--divider`, `--diagram-*` to globals.css. Load `Instrument_Serif` via next/font alongside Inter and JetBrains Mono.
2. **Move existing live feed to `/dashboard/page.tsx`.** Update all internal links (`Link` components, the dashboard CTA in `THEME.md` should already say `/dashboard`).
3. **Build `/page.tsx` shell.** Just the nav and footer. Verify scroll-state nav transition works.
4. **Hero (§5.2).** Build it first with static log-tape rows, then add the animated tape last. This is the most important section — iterate until it lands.
5. **Annotation primitives (§6).** Build `<HandArrow>` and `<Annotation>` components. Test them in isolation before placing on the page.
6. **Sections 5.3 → 5.10 in order.** Each section is a single React component in `components/landing/`. Compose them as siblings in `page.tsx`.
7. **Polish pass.** Walk top-to-bottom, fix spacing, verify mobile breakpoints, run Lighthouse.

---

## 9. Mobile / responsive

The landing is **desktop-first** because the audience is recruiters on laptops. But it must not break on mobile. Specific behaviors:

- Nav: at <640px, hide center links; show only logo + `Open ›` CTA
- Hero: headline scales to 36–40px on mobile; CTAs stack vertically; annotation is hidden (looks awkward at narrow widths)
- Before/after cards (§5.4): stack vertically
- How it works (§5.5): regions stack (already vertical, but cards inside go to 2 cols then 1)
- Pattern cards (§5.6): single column
- Code tabs (§5.7): stack code above prose
- Capabilities (§5.8): single column
- Annotations are **hidden below 900px**. They don't translate well.

Break at `768px` (md) and `1024px` (lg). Use Tailwind's defaults.

---

## 10. Forbidden on the landing page (in addition to `THEME.md` §8.4)

- **No marketing illustrations.** No abstract shapes, no isometric art, no character mascots. The only graphics are real product diagrams (SVG) and real logos.
- **No fake testimonials.** Don't invent quotes from "happy customers." It's a portfolio piece — be honest. The closing section is just a CTA, no fake social proof.
- **No animated backgrounds beyond the hero log tape.** No starfields, no grid pulses, no mesh wobbles.
- **No video.** A real demo video can live behind a "Watch demo" link on the closing CTA if you make one. No autoplay background videos.
- **No "Coming soon" sections.** Either ship it or don't mention it.
- **No counters that count up on scroll** (the capabilities §5.8 numbers are static).
- **No floating "Open dashboard" pill** in the corner on scroll. The nav stays at the top; that's enough.

---

## 11. Done checklist for the landing page

When Claude finishes the landing page, the following must all be true:

- [ ] Loads at `/` and the dashboard at `/dashboard`. All links work both directions.
- [ ] Every color is a token, no raw hex anywhere in JSX.
- [ ] No gradients, shadows, blur effects, glassmorphism, or emoji.
- [ ] Exactly two annotations are present (one in hero, one in how-it-works).
- [ ] The italic serif appears only inside annotations (and optionally one word in the H1).
- [ ] The hero's animated log tape stops animating when `prefers-reduced-motion: reduce`.
- [ ] The code tabs (§5.7) contain **real code from the actual backend**, not pseudo-code. File paths are correct, line numbers match.
- [ ] Tab switching is keyboard navigable.
- [ ] On mobile (390px): the nav collapses, annotations hide, all sections single-column, no horizontal scroll.
- [ ] Lighthouse score: Performance ≥ 90, Accessibility ≥ 95.
- [ ] The page from top to bottom feels like the same product as `/dashboard` — same colors, same fonts, same tone of voice. A reviewer should not feel like they're on a different site.
