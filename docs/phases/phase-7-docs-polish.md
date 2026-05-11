# Phase 7 — Documentation & Polish

> **Document:** `docs/phases/phase-7-docs-polish.md`
> **Owner:** Solo build
> **Created:** 2026-05-11
> **PRD reference:** §13 ("Documentation & Polish (Day 7). Final README, architecture diagram refinement, demo video, decisions.md, prompts.md, dry-run from a fresh clone, submission packaging.")

## 0. Why this phase exists

Phases 1–6 built the system. Phase 7 makes it **reviewable**. A reviewer (or future self) lands on the repo and must be able to:

1. Understand what was built in 5 minutes
2. Run the whole thing with one command
3. See proof every PRD goal was met
4. Read about every non-obvious decision

That's it. No new code. No new features. Just polish.

## 1. Scope — IN

| Deliverable | What "done" looks like |
|---|---|
| `README.md` — final pass | Phase 6 + 7 sections added; quick-start works from a fresh clone; one-line elevator pitch at the top |
| **Architecture diagram** | Mermaid diagram(s) embedded in `docs/01-architecture.md` and rendered inline in the README. Shows ingest → debounce → persist → alert plus the four stores |
| `docs/prompts.md` | Per-phase narrative of what was asked of Claude. One section per phase. Honest about what worked and what got refactored |
| `STATE.md` | Marked Phase 6 complete, Phase 7 in progress / done |
| `docs/decisions.md` | Final Phase 7 entries appended (capturing diagram choice, README structure decisions, anything else non-obvious) |
| `project-notes/README.md` | Index updated; Phase 6 + 7 Q&A files added |
| `.env.example` | Verified complete against current `main.go` env reads |
| **Dry-run** | `docker compose up` from a fresh clone reaches healthy state; `go test ./...` passes; `pnpm build` passes; `simulate-outage --scenario all` runs clean |

## 2. Scope — OUT

- Demo video (the PRD asks for one but I can't record video; I'll add a clear "what to demo" script to README instead)
- Re-running every test from scratch (already done in Phase 6 PR)
- Any feature work (out by definition)
- Touching the frontend visual design
- Refactoring backend code

## 3. The architecture diagram

Use **Mermaid** because:
- It renders inline on GitHub
- It's text → diff-able in git
- No image asset to keep in sync with the code

Two diagrams:
1. **High-level system context** — how Vellum sits between observability sources and human responders
2. **Internal runtime topology** — the four-stage pipeline + four stores + the dashboard

Both go in `docs/01-architecture.md` (replacing the ASCII art) AND a copy of (1) sits at the top of the README.

## 4. The `prompts.md` document

One section per phase. For each:
- The original PRD acceptance criteria (1 line)
- The actual prompt I gave Claude (paraphrased; this isn't a transcript dump)
- What got generated first vs. what I had to push back on
- Honest "what I'd do differently"

This is the file a reviewer reads to understand my AI-assisted process. It's also the file that proves I understood every line I committed.

## 5. README final-pass structure

```
# Vellum — Incident Management System

[one-line elevator pitch + Mermaid diagram inline]

## Quick start
[`docker compose up`, hit /dashboard]

## What this is
[3-paragraph "what" + "why"]

## Architecture at a glance
[link to docs/01-architecture.md]

## Phase acceptance (1–7)
[existing sections, plus Phase 6 + 7]

## How to demo
[curl + dashboard walkthrough proving G1, G2, G3]

## Repo layout
[unchanged]

## Tech stack
[link to docs]

## Decisions log
[link]
```

## 6. Build order

1. Phase 7 spec (this file) → DONE
2. Architecture diagram (Mermaid)
3. README final pass
4. `prompts.md`
5. `STATE.md` updated
6. project-notes Phase 6 + 7 Q&A
7. `.env.example` verification
8. `docs/decisions.md` final entries
9. Dry-run from fresh clone
10. Commit + push + PR

## 7. Acceptance criteria

- [ ] `git clone` → `docker compose up` → backend healthy → frontend at `localhost:3000`
- [ ] All Go tests pass (`go test -race ./...`)
- [ ] Frontend builds clean (`pnpm build`)
- [ ] `go run ./scripts/simulate-outage.go --scenario all` produces expected output
- [ ] README's quick-start works exactly as written
- [ ] Every phase has its acceptance section in README
- [ ] Architecture diagram renders on GitHub
- [ ] `prompts.md` has 7 sections (one per phase)
- [ ] `decisions.md` has Phase 7 entries
- [ ] STATE.md marked complete
- [ ] Project notes index updated
