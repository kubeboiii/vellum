# 00 — Master PRD: Mission-Critical Incident Management System

> **Document:** `00-master-prd.md`
> **Version:** 1.0 (draft)
> **Owner:** Engineering — Solo build

---

## 1. Document Purpose & How to Use It

This is the master Product Requirements Document for an Incident Management System (IMS) — a backend-heavy distributed system that ingests failure signals from a heterogeneous tech stack, deduplicates and tracks them as work items through a state-machine lifecycle, and exposes a workflow dashboard for human responders. The deliverable is a complete, runnable, containerized application built in seven days as part of an engineering assignment intended to demonstrate concurrency, distributed-systems design, and clean code practices.

This document is the foundation. It defines **what** is being built and **why**. It does not contain code, schemas, or step-by-step build instructions — those live in companion documents. Read this once end-to-end before opening any other document. Re-read sections 4 and 5 (functional and non-functional requirements) before reviewing any code Claude generates; they are the contract.

### Companion documents

- `01-architecture.md` — System architecture, data-flow walkthroughs, design-pattern catalog, tech-stack rationale, failure modes.
- `02-data-models.md` — Postgres DDL, MongoDB collections, Redis key patterns, TimescaleDB hypertable, protobuf messages.
- `03-api-contract.md` — Every HTTP and gRPC endpoint with request/response shapes and error semantics.
- `phases/phase-N-*.md` — Seven day-scoped build prompts, each referencing this PRD and the foundation docs.

### How to read this with Claude

When asking Claude (or any coding assistant) to build a phase, attach this PRD plus the relevant phase file plus the foundation docs it references. Do not paste the entire PRD into every prompt — it wastes context and degrades output quality. The phase files reference sections by number, e.g. "see 00-master-prd §4.3 for the debounce rule."

---

## 2. Problem Statement & Background

### 2.1 The real-world problem

Modern production systems are made of many components: REST APIs, gRPC services, MCP hosts, distributed caches (Redis clusters), async queues (Kafka, RabbitMQ), relational databases (Postgres, MySQL), NoSQL stores (Mongo, Cassandra), object storage, and external SaaS dependencies. When any of these degrade or fail, they emit signals — log entries, metrics breaches, exception traces, healthcheck failures. In a busy environment, signals arrive at thousands per second.

Three problems compound:

1. **Signal volume drowns responders.** A single bad deploy can generate tens of thousands of error events in minutes. A human on-call cannot triage at that rate.
2. **Signals do not equal incidents.** One Postgres outage produces signals from every service that talks to Postgres — APIs, async workers, cache-warmers. These are one incident, not hundreds.
3. **Incident lifecycle is unstructured.** Teams often resolve incidents without recording what happened or why. There is no audit trail, no MTTR tracking, no post-mortem discipline.

### 2.2 The system we are building

The IMS is the layer between raw observability data (logs, metrics, traces) and human responders. It ingests signals at high volume, debounces them into a small number of work items, runs each work item through a strict state-machine lifecycle that enforces a Root Cause Analysis (RCA) on closure, and exposes a dashboard for live triage.

It is not a replacement for Datadog, Grafana, PagerDuty, or Jira. It sits beside them and orchestrates the response workflow. Real signals would flow into it from those tools; real alerts would flow out of it to those tools. For this build, we mock both sides — a script simulates inbound signals and alerters log to console or a Slack webhook.

### 2.3 Why this is a useful assignment

The IMS deliberately exercises a wide surface of senior backend skills: high-throughput ingestion, backpressure, concurrency primitives, polyglot persistence, design patterns (State, Strategy), transactional integrity, async processing, retries, observability, containerization, and a small reactive frontend. A reviewer scanning the codebase and README can assess all of these in one project.

---

## 3. Goals & Non-Goals

### 3.1 Goals

- **G1.** Sustain 10,000 signals/sec ingestion without crashing, even when downstream persistence is slow or briefly unavailable.
- **G2.** Reduce noise: 100 correlated signals per component within 10 seconds collapse to a single work item, with all raw signals preserved for forensics.
- **G3.** Enforce incident-lifecycle discipline: no work item can transition to CLOSED without a complete RCA. MTTR is computed automatically.
- **G4.** Provide a responsive web dashboard for live triage, incident detail, and RCA submission.
- **G5.** Demonstrate clean separation of concerns: ingestion, debounce, workflow, persistence, alerting, and presentation are independent layers.
- **G6.** Be reproducible: one `docker compose up` command brings the entire stack to a working state.

### 3.2 Non-Goals

Calling these out explicitly prevents scope creep and prevents Claude from over-engineering.

- **NG1.** Authentication and authorization. The dashboard is open. In production this would sit behind SSO; out of scope here.
- **NG2.** Multi-tenancy. Single-org assumption.
- **NG3.** Real integrations with PagerDuty, Slack-at-scale, Datadog, or any commercial product. We stub these with console logs and a single optional Slack webhook.
- **NG4.** Production-grade horizontal scaling, leader election, distributed consensus. Single-replica per service is acceptable; the design must be ready for replication but we do not run it.
- **NG5.** Machine-learning–based correlation, anomaly detection, or predictive alerting. Debounce is purely rule-based (component-id + time window + count).
- **NG6.** Mobile app, email notifications, SMS, or any non-web client.
- **NG7.** Internationalization, accessibility audits, theming. The dashboard is English, light theme, desktop-first (responsive is a bonus).
- **NG8.** Long-term archival, GDPR-style data retention controls, encryption-at-rest tuning. Defaults are acceptable.

---

## 4. Functional Requirements

Each requirement is identified (FR-N) so phase files and test cases can reference it directly. Requirements are grouped by subsystem.

### 4.1 Signal Ingestion

- **FR-1.1** The system exposes an HTTP endpoint `POST /v1/signals` accepting a JSON payload representing a single signal.
- **FR-1.2** The system exposes a gRPC server-streaming endpoint `SignalService.IngestSignals` accepting a stream of Signal messages.
- **FR-1.3** Both endpoints share the same in-process ingestion pipeline. There is exactly one downstream path regardless of protocol.
- **FR-1.4** Each accepted signal returns immediately with a 202-equivalent acknowledgement. The caller is never blocked on database I/O.
- **FR-1.5** When the ingestion queue cannot accept further signals, callers receive `503 Service Unavailable` (HTTP) or `ResourceExhausted` (gRPC) with a retry hint. The server does not crash, leak memory, or silently drop signals without acknowledgement.
- **FR-1.6** A token-bucket rate limiter is applied per source identifier (IP for HTTP, peer for gRPC) on the ingestion endpoints. Limits are configurable; default 1000 signals/sec per source.

### 4.2 Signal Schema

Every signal carries at minimum:

- `signal_id` (UUID, generated by client or server)
- `component_id` (string, e.g. `RDBMS_PRIMARY_01`, `CACHE_CLUSTER_A`)
- `component_type` (enum: `API`, `MCP_HOST`, `CACHE`, `QUEUE`, `RDBMS`, `NOSQL`, `OTHER`)
- `severity` (enum: `P0`, `P1`, `P2`, `P3`)
- `timestamp` (ISO-8601)
- `payload` (free-form JSON object with the error/metric details)
- `source` (string identifying which observability tool emitted it)

### 4.3 Debounce Engine

- **FR-3.1** Signals are grouped by `component_id`. A debounce window is 10 seconds long and holds up to 100 signals.
- **FR-3.2** The first signal for a previously-quiet `component_id` creates a new Work Item with status `OPEN`. Subsequent signals within the window are linked to that Work Item.
- **FR-3.3** When the window expires (no new signal for 10 seconds) or the 100-signal cap is reached, the next signal opens a fresh Work Item.
- **FR-3.4** All raw signals — every single one, not just the first per window — are persisted to MongoDB and linked to their Work Item.
- **FR-3.5** Debounce state is stored in Redis using atomic operations (Lua script) so the design supports multiple ingestion-worker replicas sharing state.
- **FR-3.6** If Redis is unavailable, the system degrades to "always create a new work item" mode — noisier but functional — and logs the degradation.

### 4.4 Work Item Lifecycle

- **FR-4.1** A Work Item has four states: `OPEN`, `INVESTIGATING`, `RESOLVED`, `CLOSED`.
- **FR-4.2** Transitions are: `OPEN → INVESTIGATING`, `INVESTIGATING → RESOLVED`, `RESOLVED → CLOSED`. Backward transitions are not permitted in v1.
- **FR-4.3** Each transition is implemented via the **State design pattern**. State-specific behaviour (validation rules, side effects on entry) lives in the state object, not in a switch statement.
- **FR-4.4** Every transition writes a row to a Postgres `state_transitions` table for audit. The Work Item update and the transition row write are in a single database transaction.
- **FR-4.5** Transition to `CLOSED` is rejected unless an RCA record exists for the Work Item and the RCA is marked complete (all required fields filled). The rejection returns a structured error indicating which fields are missing.

### 4.5 Root Cause Analysis

An RCA record has the following required fields:

- `incident_start` (datetime — defaults to first signal timestamp, editable)
- `incident_end` (datetime — defaults to RCA submission time, editable)
- `root_cause_category` (enum: `CODE_DEFECT`, `INFRASTRUCTURE`, `CONFIG_CHANGE`, `EXTERNAL_DEPENDENCY`, `CAPACITY`, `HUMAN_ERROR`, `OTHER`)
- `fix_applied` (text, min 20 chars)
- `prevention_steps` (text, min 20 chars)
- `submitted_by` (string, user identifier — defaults to "system" if no auth)

MTTR is computed automatically as `incident_end - incident_start` and stored on the Work Item at the moment of CLOSE.

### 4.6 Alerting

- **FR-6.1** On Work Item creation, an alert is dispatched using a **Strategy** selected by `component_type` and `severity`.
- **FR-6.2** Strategies implemented for v1:
  - `PagerDutyStub` — P0 only, logs to console with structured fields simulating a PD payload
  - `SlackWebhook` — P1, P2 — sends real HTTP POST to a configured URL if present, otherwise logs
  - `Console` — P3, all others, fallback
- **FR-6.3** Strategies are interchangeable at runtime via a registry. Adding a new alerter is a one-file change.
- **FR-6.4** Alert dispatch is async and isolated — a failing alerter cannot block ingestion or workflow.

### 4.7 Dashboard UI

- **FR-7.1** Live feed page: list of active (non-CLOSED) Work Items, sorted by severity then most-recent-signal-time descending. Auto-refreshes every 2 seconds.
- **FR-7.2** Incident detail page: full Work Item record, complete list of linked raw signals (paginated, default 50/page), state-transition history, current alert dispatches.
- **FR-7.3** RCA form page: a form for the fields in §4.5. Validates client-side and server-side. On successful submission, the Work Item moves to `CLOSED`.
- **FR-7.4** State-transition controls: from the detail page, the user can advance the Work Item to the next valid state (one button per allowed transition). Invalid transitions are not shown.
- **FR-7.5** The UI never reads from Postgres directly. All data flows through the backend API, which serves the dashboard from Redis where possible (live feed) and from Postgres/Mongo where necessary (detail page).

### 4.8 Observability

- **FR-8.1** `GET /health` returns 200 with a JSON body summarizing the status of each downstream dependency (Postgres, Mongo, Redis, TimescaleDB) and the current ingestion queue depth. Returns 503 if any critical dependency is down.
- **FR-8.2** Every 5 seconds, the backend prints to stdout a metrics line including: signals/sec accepted, signals/sec processed, current queue depth, debounce hit rate, work items created in the last interval, error rate.
- **FR-8.3** All DB writes are instrumented with retry-on-transient-failure (exponential backoff, 3 attempts, then dead-letter to a Mongo `dead_letter` collection).

---

## 5. Non-Functional Requirements

### 5.1 Performance

- **NFR-1.1** Sustain 10,000 signals/sec for 60 seconds with p99 ingestion latency under 50ms.
- **NFR-1.2** Live feed API responds in under 100ms at p99 under nominal load.
- **NFR-1.3** Incident detail page loads (full render) in under 500ms for incidents with up to 5,000 linked signals.

### 5.2 Resilience

- **NFR-2.1** Backpressure: when the in-memory queue is full, ingestion returns 503/`ResourceExhausted` within 5ms. No callers ever wait indefinitely.
- **NFR-2.2** DB writes retry with exponential backoff (base 100ms, max 3 attempts) before dead-lettering. Logs every retry.
- **NFR-2.3** If Redis is unavailable, debounce degrades gracefully; if Postgres is unavailable, ingestion continues into Mongo (raw audit) and Postgres writes queue with retry.
- **NFR-2.4** Graceful shutdown: on SIGTERM, the server drains the in-memory queue with a 30-second deadline before exiting.

### 5.3 Code Quality

- **NFR-3.1** Idiomatic Go: small interfaces, dependency injection via constructors, no global state except metrics counters. `gofmt`-clean, `go vet`-clean.
- **NFR-3.2** Unit tests for: debounce logic, state-machine transitions (every transition + every rejection), RCA validation, MTTR calculation. Target ≥60% coverage on core packages.
- **NFR-3.3** Integration test: end-to-end signal-through-RCA flow exercised against ephemeral containers.

### 5.4 Operability

- **NFR-4.1** All services run via `docker compose up` from the repository root. No host-machine dependencies beyond Docker.
- **NFR-4.2** Configuration via environment variables, with sane defaults. A `.env.example` is checked in.
- **NFR-4.3** Structured logging (JSON) to stdout. Log level configurable. No file-based logs.

---

## 6. User Personas & Journeys

Three personas use the IMS. We do not implement auth, but the personas shape what the UI prioritizes.

### 6.1 The on-call SRE

Wakes up at 03:00 to a pager. Opens the dashboard. Needs, in this order:
1. What is broken (component, severity)
2. Is it spreading (signal rate)
3. Who else is looking at it (assignee)
4. What do the raw errors say (signal payloads)

**Journey:** opens live feed → sorts by P0 → clicks top incident → reads the most recent 10 signal payloads → moves Work Item to `INVESTIGATING` and starts diagnosing in their actual systems.

### 6.2 The incident commander

Coordinates the response across responders. Cares about timeline and current status of every active incident, not the raw error details.

**Journey:** live feed open in one tab, watches the count of `OPEN` vs `INVESTIGATING` vs `RESOLVED`. When something moves to `RESOLVED`, prompts the responder to write the RCA.

### 6.3 The post-mortem author

Writes the RCA after the dust settles. Cares about: start/end times, what signals fired and when, what category of failure this was, what fixed it, what will prevent recurrence.

**Journey:** opens `RESOLVED` incident → reviews state-transition timeline → fills RCA form → submits → Work Item moves to `CLOSED`, MTTR is computed and shown.

---

## 7. Success Criteria & How We Demonstrate Them

Each goal from §3.1 maps to a verifiable demonstration. These doubles as the demo script for the README video.

| Goal | Demonstration | Pass criterion |
|---|---|---|
| **G1** | Run vegeta load test at 10K req/s for 60s against `/v1/signals` | Server stays up; 99%+ requests return 202; metrics line shows sustained throughput; p99 latency under 50ms |
| **G2** | Run failure simulator: 200 signals to `CACHE_CLUSTER_01` in 8 seconds | MongoDB shows ~200 raw signals; Postgres shows 1-3 work items (depending on window boundaries); reduction ratio ≥ 60× |
| **G3** | Try to PATCH a Work Item to `CLOSED` without an RCA; then with incomplete RCA; then with complete RCA | First two attempts return 422 with field-level errors. Third succeeds; MTTR appears on the response payload |
| **G4** | Open dashboard during simulator run, walk through live feed → detail → RCA submission | All three pages function; live feed updates within 2s; RCA submission closes the incident |
| **G5** | Code review: locate ingestion, debounce, workflow, persistence, alerting in distinct packages | Each subsystem has its own package; cross-package dependencies flow inward (alerting depends on workflow, workflow on persistence; not the reverse) |
| **G6** | Clone repo on a clean machine with only Docker; run `docker compose up` | Stack reaches healthy state within 90 seconds. Dashboard accessible at `localhost:3000`. `/health` returns 200. |

---

## 8. Glossary

These terms are used consistently throughout all project documents and in code. Pin them down here.

| Term | Definition |
|---|---|
| **Signal** | A single observation that something might be wrong: an error log, a metric breach, an exception trace. Many signals per second. Stored in Mongo as the raw audit log. |
| **Work Item** | The aggregated, actionable unit. One Work Item represents one ongoing incident on one component. Created by the debouncer. Stored in Postgres. Has a lifecycle (state machine). |
| **Incident** | Loose synonym for Work Item in user-facing copy. Code uses "Work Item." UI uses "Incident." Don't mix them in the same context. |
| **RCA** | Root Cause Analysis. A structured record explaining a closed Work Item: timeline, category, fix, prevention. Required for `CLOSED` status. |
| **MTTR** | Mean Time To Repair. For a single incident: `incident_end - incident_start`. Computed at close time, stored on the Work Item. |
| **Debounce window** | The 10-second period after the first signal for a component during which subsequent signals attach to the same Work Item rather than spawning new ones. Capped at 100 signals. |
| **Component** | A monitored unit of the production stack: a database instance, a cache cluster, an API service, an MCP host. Identified by `component_id`. Has a `component_type`. |
| **Alerter** | A Strategy implementation that dispatches a notification when a Work Item is created. `PagerDutyAlerter`, `SlackAlerter`, `ConsoleAlerter`. |
| **Sink** | A persistence destination. We have four: Mongo (raw signal audit), Postgres (transactional work items + RCA), Redis (dashboard hot cache + debounce state), TimescaleDB (timeseries aggregations). |
| **Backpressure** | The mechanism by which a slow downstream stage signals to an upstream stage to slow down. Here: a bounded channel between ingestion and workers; when full, ingestion returns 503. |
| **Hot path** | The code path executed on every incoming signal. Performance-critical. Must not do synchronous I/O against slow backends. |
| **Dead letter** | A storage location for signals or work items that failed all retry attempts. We use a Mongo collection. Inspected manually; not auto-replayed in v1. |

---

## 9. Assignment Rubric Mapping

The assignment evaluates seven categories. This section maps each rubric line to where in this PRD and the project it is satisfied. Treat this as the scoring checklist.

| Rubric category | Weight | Where satisfied |
|---|---|---|
| **Concurrency & Scaling** | 10% | FR-1.4, FR-1.5, NFR-1.1, NFR-2.1. Bounded channel ingestion, worker pool, atomic Redis Lua for debounce, transactional Postgres for state changes. Demonstrated in Phase 2 load test. |
| **Data Handling** | 20% | FR-3, FR-4. Four distinct sinks with clear purposes; Mongo for raw audit, Postgres for transactional truth, Redis for hot cache + debounce, TimescaleDB for timeseries. Detailed in `02-data-models`. |
| **LLD** | 20% | FR-4.3 (State pattern), FR-6.3 (Strategy pattern). Small interfaces, dependency injection. Detailed in `01-architecture` §7. |
| **UI/UX & Integration** | 20% | FR-7. Three Next.js pages backed by clean API contract (`03-api-contract`). Live feed, detail, RCA form. |
| **Resilience & Testing** | 10% | FR-8.3 (retry), NFR-2 (resilience), NFR-3.2 (tests). Retry-with-backoff on DB writes; dead-letter; unit tests for state machine and RCA validation; integration test in Phase 6. |
| **Documentation** | 10% | This PRD + 01-03 foundation docs + phase files + `README.md` + `decisions.md` + `prompts.md`. Submission asks for all of these explicitly. |
| **Tech Stack Choices** | 10% | `01-architecture` §3.2 tech-stack table with rationale per choice. Goes beyond "because I know it" — each choice tied to a requirement. |

---

## 10. Risks & Open Questions

### 10.1 Risks

- **R1. Scope creep eats time.** The system is large for a week. Every "nice to have" must be pushed to the bonus list. Hard cut-off: if Day 5 is behind, scrap any unfinished bonus features and use Day 6-7 entirely for testing, docs, and the demo.
- **R2. Concurrency bugs hide in tests.** Race conditions in the debouncer and state machine will not appear under low load. Mitigation: load test by Day 3, run with `-race` flag always, write at least one concurrency-focused test (N goroutines, same `component_id`, assert exactly one work item).
- **R3. AI-generated code without understanding.** Every phase file ends with "learning notes — you must be able to explain X." If you cannot, do not merge. Interview risk is real.
- **R4. Frontend takes longer than estimated.** Mitigation: use shadcn/ui components, no custom design. Three pages, not five. Defer animation/polish to bonus.
- **R5. Docker compose works on your laptop but fails on the reviewer's.** Mitigation: test from a fresh git clone on Day 7. Pin all image versions. Document the exact `docker compose` invocation.

### 10.2 Open questions (resolve before Phase 2)

- Do we want signal_id collision detection (dedupe identical signal_ids received twice)? Probably no for v1.
- Should the debouncer support escalation (e.g. if signal count > 500 in a window, auto-promote severity)? Probably no for v1, mark as bonus.
- Do we expose a way to manually merge two work items? Probably no for v1, mark as bonus.
- Should RCAs support comments / collaborative editing? No, single-author for v1.

---

## 11. Bonus Features (Time Permitting)

The assignment notes "Bonus points for any creative additions done." These are explicit candidates. None are required. Pick at most two; only if Day 5 finishes early.

- **B1.** Severity escalation: if a single Work Item accumulates over 500 signals or persists `OPEN` > 5 minutes, automatically bump its severity and re-fire the alert with the new strategy.
- **B2.** WebSocket live updates: replace 2-second polling on the dashboard live feed with a WebSocket push from the backend when Redis state changes.
- **B3.** Time-travel debugging: a dashboard page that replays the signal timeline of a closed incident, with a scrubber.
- **B4.** Alerter dry-run: a dashboard toggle that runs alerts in "shadow mode" — logs what would have been sent without actually firing webhooks. Useful when reviewers run the system.
- **B5.** RCA template suggestions: based on `root_cause_category`, pre-fill the `fix_applied` and `prevention_steps` with a starter template.
- **B6.** Grafana dashboard wired to TimescaleDB showing signal rate, MTTR distribution, debounce ratio. Shipped as a sidecar container in docker-compose.

---

## 12. Tech Stack at a Glance

Full rationale lives in `01-architecture`. Quick reference here.

| Layer | Choice | Reason (one-liner) |
|---|---|---|
| Backend language | Go 1.22+ | Goroutines and channels map cleanly to the ingestion model; static binary; fast cold start |
| HTTP framework | Gin | Lightweight, ergonomic middleware (rate limiter), large community |
| RPC | gRPC + protobuf | Streaming RPC for high-volume internal signal sources; strongly-typed contract |
| Transactional store | PostgreSQL 16 | Work items and RCAs need ACID + foreign keys + JSON columns; SERIALIZABLE for state transitions |
| Audit log store | MongoDB 7 | Schema-flexible payloads; high-volume append-only; indexable on `component_id + timestamp` |
| Cache + debounce | Redis 7 | Atomic Lua scripts for debounce; sorted sets for live feed; sub-millisecond reads |
| Timeseries | TimescaleDB (Postgres extension) | Hypertables for signal/MTTR aggregates; one less moving part than Prometheus |
| Postgres driver | pgx v5 | Native Postgres protocol, faster and richer than `database/sql`; built-in pooling |
| Frontend | Next.js 14 (App Router) | Server components for live feed (revalidation), client components for forms; one toolchain |
| UI library | shadcn/ui + Tailwind | Composable, owned components; no design-from-scratch |
| Orchestration | Docker Compose | Single command bring-up; pinned versions; reviewer-friendly |
| Load test | vegeta | Scriptable, generates clean reports, scales to 10K+ rps from a single host |

---

## 13. Build Phases at a Glance

Each phase is one calendar day. Each has its own phase file. This is the high-level view.

- **Phase 1 — Foundation (Day 1).** Repo scaffolding, Docker Compose with all four databases running, empty Go module, empty Next.js app, README skeleton, architecture diagram. No application logic yet.
- **Phase 2 — Ingestion & Backpressure (Day 2).** HTTP endpoint, bounded channel, worker pool, token-bucket rate limiter, throughput metrics ticker, `/health` endpoint. Load test proves 10K signals/sec.
- **Phase 3 — Debounce & Persistence Fan-out (Day 3).** Redis Lua debounce script, persistence to Mongo (raw signals) and Postgres (work items), retry-with-backoff, dead-letter, TimescaleDB writes for timeseries.
- **Phase 4 — Workflow Engine (Day 4).** State pattern for Work Item lifecycle, Strategy pattern for alerters, RCA model and validation, MTTR calculation, transactional state transitions, unit tests.
- **Phase 5 — gRPC + Frontend (Day 5).** Add gRPC streaming endpoint sharing the same ingestion pipeline. Next.js dashboard with live feed, incident detail, RCA form.
- **Phase 6 — Resilience & Simulation (Day 6).** Failure simulator script (cascading RDBMS → cache → MCP failure scenario). Integration test. Concurrency stress test. Bug-fixing pass.
- **Phase 7 — Documentation & Polish (Day 7).** Final README, architecture diagram refinement, demo video, `decisions.md`, `prompts.md`, dry-run from a fresh clone, submission packaging.

---

## 14. Sign-off & Change Control

This is a living document but it stabilizes on Day 1. Any change to a requirement after Day 2 must be recorded in `/docs/decisions.md` with date, rationale, and impact on phase files.

Hard rule: do not skip ahead. Phase N's acceptance criteria must be met before Phase N+1 starts. The only exception is documentation, which can run in parallel.

> *If you cannot explain it on a whiteboard, you do not own it. Slow down.*
