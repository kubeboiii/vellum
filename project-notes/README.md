# Project Notes — Vellum Study Guide

> **Purpose:** A phase-by-phase Q&A walkthrough of every design choice in
> the Vellum project. Written for someone learning backend development who
> wants to actually understand the code, not just ship it. Also doubles
> as interview prep — every Q is something a senior reviewer might ask.

## How to use this

1. **Before reading code** for a phase, skim the phase's Q&A doc to
   anchor the vocabulary.
2. **After implementing** the phase, come back and read it slowly. The
   answers should now feel obvious — if they don't, the implementation
   probably has a soft spot.
3. **Before an interview**, the "Interview gotchas" section at the
   bottom of each phase doc is the cheat sheet.

## Reading order

| # | File | Covers |
|---|---|---|
| 1 | [phase-1-foundation-qa.md](phase-1-foundation-qa.md) | Repo scaffolding, Docker, the 4 data stores, why each one, Go modules, Next.js basics |
| 2 | [phase-2-ingestion-qa.md](phase-2-ingestion-qa.md) | Goroutines & channels, backpressure, rate limiting, worker pools, /health, vegeta load testing |
| 3 | [phase-3-debounce-qa.md](phase-3-debounce-qa.md) | Redis Lua atomic ops, polyglot persistence fan-out, retry+backoff, dead-letter, hypertables, testcontainers |
| 4 | [phase-4-workflow-qa.md](phase-4-workflow-qa.md) | State pattern, Strategy pattern, RCA validation, MTTR, SERIALIZABLE transactions, SELECT FOR UPDATE |
| 5 | [phase-5-grpc-frontend-qa.md](phase-5-grpc-frontend-qa.md) | gRPC bidi-streaming, protobuf, Next.js Server vs Client Components, CORS, SQLSTATE 40001 → 409 |
| 6 | [phase-6-resilience-qa.md](phase-6-resilience-qa.md) | Concurrency stress testing, integration testing strategy, build tags, headless simulator design |
| 7 | *(no Q&A — Phase 7 was docs/polish only; see `docs/prompts.md` for the narrative)* | |

## How each doc is structured

Every phase Q&A follows the same shape:

1. **What we built** — one-paragraph summary, no jargon.
2. **The fundamentals** — language/runtime/stack concepts you need to
   know before the design choices make sense.
3. **The tech we used** — what each library/tool is, in your own words.
4. **The design decisions** — every "why this, not that" Q.
5. **Tradeoffs** — what we gave up.
6. **Interview gotchas** — the 10-ish Qs most likely to come up.

## A note on style

Answers aim for **short and specific**. If you want longer prose, the
relevant PRD/architecture section is linked. The goal here is *recall*,
not first-time learning — pair each doc with the corresponding code and
the foundation docs (`docs/00-master-prd.md`, `docs/01-architecture.md`).
