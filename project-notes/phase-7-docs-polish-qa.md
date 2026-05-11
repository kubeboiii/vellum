# Phase 7 — Documentation & Polish: Q&A Study Guide

> Companion to `docs/phases/phase-7-docs-polish.md`. The shortest study guide of the seven because Phase 7 doesn't ship code — it ships clarity.

## What we built

No code. Polish: Mermaid diagrams, a Phase 6 + 7 README pass, `docs/prompts.md`, this Q&A file, an audit of `.env.example`, and a fresh-clone dry-run that proves the whole stack works from `git clone`.

## The fundamentals

### Why does documentation matter for a portfolio project?
A reviewer has ~10 minutes per project. They cannot read all the code. They CAN read:
- The README's first screen
- The architecture diagram
- The decisions log
- A demo (recorded or click-through)

If those four are good, the code review is "spot-check the State pattern" — fast and favourable. If those four are bad, the reviewer goes elsewhere.

### What's a "fresh-clone dry-run"?
You `git clone` the repo to a clean directory, follow the README's quick-start verbatim, and verify everything works. This catches:
- README assumes a tool is installed (it isn't)
- README's `docker compose up` line is wrong
- An `.env` value is hardcoded somewhere
- Submodules aren't checked in

PRD §10 R5: "Docker compose works on your laptop but fails on the reviewer's."

## The tech we used

### Mermaid
A text-based diagram format that GitHub, GitLab, and most modern markdown renderers compile to SVG inline. Diff-able in git, no image files to keep in sync. Trade: limited layout control vs. tools like Lucidchart, but the diagrams are versioned with the code.

### `git diff --stat`
Shows the file-level summary of a commit. Useful for understanding "what changed in this PR" without reading the diff line by line.

### `gh pr create`
GitHub CLI command to open a PR from the command line. Authenticated via `gh auth login`. Used to open PR #6 (Phase 5), #7 (Phase 6), and #8 (Phase 7).

## The design decisions

### Q1. Why Mermaid instead of an SVG export from a design tool?
- **Diff-able**: a PR that updates the diagram shows the change in the actual code review.
- **Renders on GitHub**: no asset to upload, no CDN to manage.
- **Trade**: less layout control. For a system context + runtime topology, this is enough.

### Q2. Why isn't there a demo video?
The PRD asks for one. The right answer is: "I can't record video reliably from this environment." The fallback is a thorough "How to demo" section in the README with the three commands that prove G1, G2, G3 plus the dashboard walkthrough. A reviewer can record their own video.

### Q3. Why `prompts.md` instead of a transcript dump?
A transcript dump of every Claude prompt would be 50+ pages of mostly-noise. The reader wants the **process** — what was asked, what came out, what got pushed back. That's a narrative, not a log.

### Q4. Why update STATE.md throughout the build, not just at the end?
Each session reads STATE.md first to know where the project is. Without it, every new session starts from scratch. The "Last session ended" sentence is the most valuable token in the entire repo.

### Q5. Why a separate `project-notes/` directory instead of just docs/?
- `docs/` is for the PRD, architecture, API contract, decisions, prompts, and phase specs. These are deliverables.
- `project-notes/` is for **learning**. The Q&A docs are scaffolding for the build, not part of the final artefact. A reviewer can skip them entirely.

### Q6. Why so many `decisions.md` entries (27)?
Every entry exists because the choice would otherwise look arbitrary or wrong. Examples:
- "Why pgx instead of database/sql" — looks like a preference, isn't (driver features).
- "Why CategoryBreakdown is lazy-loaded" — looks like laziness, isn't (avoids 100 round-trips on page load).
- "Why we kept StatCards alongside SeverityStackedBar" — looks redundant, isn't (different questions).

The 27 entries serve as the **defence-in-depth** for the rubric's "Documentation" 10%.

## Tradeoffs

- **No video.** Trade: lower production value vs. environmental reliability. Mitigated by the "How to demo" section.
- **Mermaid's layout is approximate.** Trade: diff-ability vs. polish. For system diagrams, fine.
- **`prompts.md` is narrative, not exhaustive.** Trade: readability vs. completeness. A reviewer wouldn't read a 50-page transcript.

## Interview gotchas

### "Walk me through the architecture."
Open the README. Use the Mermaid diagram at the top. Three sentences:
- "10K signals/sec come in via HTTP or gRPC. Both go through a bounded channel and a worker pool. Backpressure returns 503 when the channel is full."
- "The debouncer is a Redis Lua script that atomically groups signals by component_id into work_items, with a 100-signal cap per window."
- "Work items go through a State-pattern lifecycle. CLOSED is unreachable without a valid RCA, enforced in one place."

### "How did you use AI?"
Open `docs/prompts.md`. Walk through the 7 phases. Be specific: "Phase 4 — I asked Claude for the State pattern, pushed back on the SQL isolation level (it gave READ COMMITTED, I asked for SERIALIZABLE), and added the SQLSTATE 40001 → 409 mapping."

The honest answer separates "AI did all this for me" from "AI accelerated me; I steered." The PRD's risk R3 is "AI-generated code without understanding" — addressing it directly is a strength.

### "What would you build next?"
Three things, in priority:
1. WebSocket push to replace 2s polling (PRD §11 B2)
2. Real Slack webhook integration (the stub is one Go function away from real)
3. A `/health` endpoint shape that includes counters, so the dashboard can show ingest rate without polling `/v1/incidents` (avoid the synthetic rate computation in `SignalRateChart`)

### "What's the riskiest piece of code in the project?"
The Lua debounce script. It's 30 lines but it's the atomic primitive everything else depends on. A bug there silently breaks G2 and any work_item-ordering guarantee. Mitigated by:
- The script in its own file (no string-literal escaping bugs)
- Loaded at startup with `SCRIPT LOAD` (any syntax error is a startup crash, not a runtime one)
- Two stress tests across windows and boundaries

### "What's the weakest part of the system?"
The frontend's reliance on 2-second polling. Three improvements (in priority order):
1. WebSocket push for live feed updates (PRD §11 B2 bonus)
2. SSE for the dashboard's lower-priority components (HealthStrip, IncidentRateStrip)
3. A `/v1/metrics/rate` endpoint that returns the real time-bucketed signal rate from TimescaleDB instead of the client-synthesised approximation in `SignalRateChart`

### "Why no auth?"
PRD NG1: explicitly out of scope. The dashboard is open. In production this would sit behind SSO. The system would need to add a `user_id` field on `state_transitions.actor` and `rca.submitted_by`, both of which already exist. So adding auth is additive, not invasive.

### "What's the most over-engineered piece?"
The persona switcher on `/dashboard`. Three personas, persisted in localStorage, with role labels and descriptions. The PRD §6 says "personas shape what the UI prioritizes" — I implemented it literally. A simpler product would just default to the SRE view and never expose the toggle.

### "What's the most under-engineered piece?"
The alerter Strategy. Today it fires off Slack webhooks fire-and-forget; failure is logged but not persisted. A real product would have an `alert_dispatches` table with status, retry count, response body. Listed as Tier-3 future work in `docs/phases/phase-5-dashboard-buildout.md`.
