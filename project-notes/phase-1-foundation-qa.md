# Phase 1 — Foundation: Q&A Study Guide

> **Phase scope:** Repo scaffolding, Docker Compose with Postgres+TimescaleDB,
> MongoDB, and Redis, a Go module that boots an empty Gin server on `:8080`,
> a Next.js 14 frontend skeleton, and a README. No application logic.

---

## 1. What we built (one paragraph)

We laid down the project's skeleton: a Go backend module (`backend/`), a
Next.js frontend (`frontend/`), and a Docker Compose file (`docker/compose.yaml`)
that brings up the four data stores the system will need. Nothing in the
backend "does" anything yet — the only HTTP endpoint is a placeholder
`/health` that returns 200. The point of this phase is that anyone with
Docker + Go + Node can clone the repo and have a working dev environment
inside a minute.

---

## 2. The fundamentals

### Q: What is a "monorepo" and is this one?
**A:** A monorepo holds multiple deployable units (backend, frontend,
scripts, infrastructure) in a single git repository. Yes, this is a
monorepo — `backend/` and `frontend/` are siblings. The alternative is
"polyrepo" (one repo per service); monorepo wins for small teams because
a change that touches both sides is one PR, not two.

### Q: What is Go (golang) and why pick it for the backend?
**A:** Go is a compiled, statically-typed language from Google. We picked
it for three reasons:
- **Goroutines + channels** — concurrency primitives that map perfectly
  to the "ingest signals concurrently" requirement. (Phase 2 leans on this.)
- **Single static binary** — `go build` produces one file with no
  runtime dependency. Easy to put in a Docker image.
- **Fast cold start** — milliseconds, not seconds (vs. JVM-based Java/Kotlin).

### Q: What is `go.mod` and what does `module github.com/kubeboiii/vellum` mean?
**A:** `go.mod` is Go's package manifest (like `package.json` for Node).
The `module` line declares the *import path* — anywhere in this project,
`internal/pipeline` is imported as `github.com/kubeboiii/vellum/internal/pipeline`.
The string doesn't have to point to a real GitHub repo; it just has to
be globally unique among Go modules.

### Q: What's `internal/` for?
**A:** Go's module system treats `internal/` specially: packages under
`internal/` can **only be imported by code in the same module**. It's a
compiler-enforced "private to this project" boundary. Anything you don't
want third parties depending on goes in `internal/`.

### Q: What is Docker, and what's a "container"?
**A:** Docker packages an application plus everything it needs to run
(OS libraries, language runtime, dependencies) into an *image*. A
*container* is a running instance of that image, isolated from the host
OS like a very lightweight VM. The benefit: "works on my machine" stops
being a thing — if it runs in the container on your laptop, it runs in
the container on a server.

### Q: What's Docker Compose vs. plain Docker?
**A:** Plain Docker runs one container at a time (`docker run …`). Compose
is a YAML file that describes *multiple* containers and how they talk
to each other, brought up with one command (`docker compose up`).
Compose is for dev/local; production usually uses Kubernetes or similar.

### Q: What does `docker compose up` actually do here?
**A:**
1. Reads `docker/compose.yaml`.
2. For each `service:` block, pulls the image if needed, creates a
   container, and starts it.
3. Creates a private bridge network so containers can reach each other
   by service name (e.g., `postgres` resolves to the Postgres container's IP).
4. Mounts the named volumes (`postgres-data`, `mongo-data`) for persistence.
5. Runs each container's `healthcheck` until it reports `healthy`.

### Q: What's a "healthcheck" in Compose?
**A:** A command the container runs periodically to prove it's actually
ready to serve traffic, not just running. For Postgres we use
`pg_isready`; for Redis, `redis-cli ping`; for Mongo, `mongosh ... ping`.
Without a healthcheck, "running" only means "the process started" — a
DB might still be replaying WAL and refusing connections.

### Q: What is Next.js, and what's the "App Router"?
**A:** Next.js is a React framework that bundles routing, server-side
rendering (SSR), and a build pipeline. The **App Router** is the newer
of its two routing systems (the older is the Pages Router). App Router
uses the filesystem under `app/` to define routes — `app/page.tsx` is
`/`, `app/incidents/[id]/page.tsx` is `/incidents/123`, etc.

### Q: What is pnpm and why not just npm?
**A:** pnpm is a faster, more disk-efficient alternative to npm. It uses
a global content-addressable store for packages, so installing the
same package twice across projects only stores it once. CLAUDE.md
specifies pnpm; we installed it via `corepack` (which ships with Node).

---

## 3. The tech we used

### Q: What is PostgreSQL ("Postgres")?
**A:** A mature open-source relational database (SQL). It's our **source
of truth** for work items, RCAs, and state transitions. We picked it for
ACID guarantees (transactions that are all-or-nothing) and rich features
like JSON columns and foreign keys.

### Q: What is TimescaleDB?
**A:** A Postgres **extension** (not a separate database) that adds
"hypertables" — tables auto-partitioned by time. Great for timeseries
data (signal rates, MTTR over time, etc.). Because it's an extension,
it shares the same connection pool, the same driver, the same
backup story as Postgres. That's why we have one Postgres container,
not two.

### Q: What is MongoDB?
**A:** A document-oriented NoSQL database. Each "row" (called a *document*)
is a JSON-like object that doesn't need to match a fixed schema. We use
it as the **audit log** for raw signals: each signal can have wildly
different payload shapes (a Datadog metric vs. a Sentry exception vs. a
custom healthcheck), so a schemaless store is a natural fit.

### Q: What is Redis?
**A:** An in-memory key-value store. Everything lives in RAM (with optional
disk persistence), making reads and writes sub-millisecond. We use it
for two things in Vellum: (a) the **debounce window** (Phase 3 — atomic
check-then-act via Lua scripts) and (b) a hot cache for the dashboard's
live feed.

### Q: What's Gin?
**A:** A small HTTP framework for Go. It gives you routing, middleware,
JSON helpers, and a context object — none of which the standard library's
`net/http` has out of the box. We use it for `/v1/signals` and `/health`.
Alternatives are Echo, Fiber, Chi.

### Q: Why pin Docker image versions (e.g., `redis:7.4.1-alpine`)?
**A:** If you write `redis:7`, the image you get changes every time
Docker pulls (whatever they last tagged `:7`). Pinning to `7.4.1`
guarantees the same bits today and a year from now. **Reproducibility**
is the whole point of `docker compose up` — it falls apart if the images
silently drift.

### Q: What's `:alpine` mean in `redis:7.4.1-alpine`?
**A:** Alpine Linux is a stripped-down Linux distribution (~5 MB). Images
based on it are 10–20× smaller than ones based on Debian/Ubuntu. Smaller
images = faster pulls, less surface area for security issues. We use
`alpine` for Redis (which is just a single binary, doesn't need a full OS).

---

## 4. The design decisions

### Q: Why are Postgres + TimescaleDB in the same container?
**A:** TimescaleDB is a Postgres *extension*, not a separate engine. The
`timescale/timescaledb:2.17.2-pg16` image is "Postgres 16 with the
extension preloaded". Splitting into two containers would:
- Double the operational surface (two containers to monitor, back up).
- Add a network hop for every cross-store query.
- Mean two different drivers in Go code.

One container is the standard deployment pattern and saves a moving part.
This is logged in `docs/decisions.md`.

### Q: Why three containers for "4 databases"?
**A:** The four are *logical stores* (Postgres, TimescaleDB, MongoDB,
Redis), but TimescaleDB rides in the Postgres container as an extension.
That's: Postgres+Timescale = 1 container, Mongo = 1 container, Redis = 1
container. README calls this out explicitly so a reviewer doesn't
miscount.

### Q: Why have an `init.sql` for Postgres?
**A:** `CREATE EXTENSION timescaledb` requires superuser privileges. Doing
it in `docker-entrypoint-initdb.d/00-init.sql` runs it once, as the
container's superuser, on first boot. Application-level migrations (the
ones in `backend/migrations/`) can then run as a least-privileged user.

### Q: Why `.keep` files in empty package directories?
**A:** Git doesn't track empty directories. If we want `backend/internal/ingest/`
to exist in the repo even before there's any Go code in it, we drop a
zero-byte `.keep` file in there. (When Phase 2 added real `.go` files,
we deleted the `.keep`.)

### Q: Why a `.gitignore` and what's in it?
**A:** Lists patterns Git should never stage. Ours excludes:
- `node_modules/` — JS deps, regenerated by `pnpm install`.
- `.next/` — Next.js build output.
- `coverage.out` — Go test coverage artifacts.
- `.env` — local secrets (we ship `.env.example` instead).
- `.DS_Store` — macOS metadata.

### Q: Why a `.env.example` instead of just `.env`?
**A:** `.env` typically holds real secrets and is gitignored. `.env.example`
is the *template* you commit, with the variable names but fake values.
Anyone cloning copies it: `cp .env.example .env` and fills in real values.

### Q: Why pin a graceful-shutdown handler in `cmd/vellum/main.go` even in Phase 1?
**A:** Practicing the pattern from day one. When SIGTERM hits, we have
~10s to finish in-flight requests and exit cleanly. Phase 2+ extends
this to also drain the worker pool, but the structure was right from
the start.

---

## 5. Tradeoffs

### Q: We're not using Kubernetes. Why?
**A:** K8s solves problems we don't have (multi-node scheduling, rolling
deploys, autoscaling). For a single-developer build that runs on one laptop,
Compose is the right tool. Adding K8s would be over-engineering and would
double the things to explain in an interview.

### Q: We picked PostgreSQL over MySQL. Why?
**A:** Both are mature SQL DBs; either would work. Postgres wins for us
because:
- Better JSON support (`jsonb` type with full indexing).
- TimescaleDB extension means we don't need a separate timeseries DB.
- Stricter SQL standard compliance (catches more bugs at query-write time).

### Q: Why one HTTP server (Gin) and not microservices?
**A:** The whole system is one Go binary. Microservices would mean
ingestion, workflow, and alerting in separate processes communicating
over HTTP/gRPC — more failure modes, more moving parts, harder to debug.
The architecture doc (§3.1) defends this: the channel between handlers
and workers can later become a Kafka topic if we wanted to split them,
but for now one process is correct.

---

## 6. Interview gotchas

> *These are the Phase 1 Qs most likely to come up in an interview. Lean
> answers; expand from the sections above if pressed.*

**1. "Why polyglot persistence? Couldn't you do this with just Postgres?"**
Yes, technically — Postgres has JSONB and time-partitioned tables. But:
each store is *purpose-built* for one of our access patterns. Mongo
ingests 10K/sec of variable-schema documents better than Postgres JSONB
indexes; Redis answers dashboard live-feed reads in <1ms; Postgres handles
transactional state machine writes. Using the right tool for each access
pattern is the design lesson the assignment grades.

**2. "Why pin image versions to patch (not just major)?"**
Reproducibility. `:7` is whatever Redis last tagged that way — could
change between my clone and yours. `:7.4.1-alpine` is the exact bits we
developed and tested against. Cost: manual bumps. Worth it for a demo.

**3. "Why a healthcheck and not just 'is it running'?"**
A process can be running but not ready (Postgres replaying WAL, Mongo
electing a primary). Healthchecks differentiate "process up" from
"ready to serve". `pg_isready`, `redis-cli ping`, `mongosh --eval ping`
are the cheap, standard probes.

**4. "Why one Postgres container for both transactional and timeseries data?"**
TimescaleDB is a Postgres extension. Two stores would double connections,
backups, monitoring. One pool, one driver, one backup — same isolation
because hypertables are independent tables under the hood.

**5. "What stops me from putting all the secrets in `.env` and committing it?"**
Nothing technically — the gitignore does, but only if you trust the
convention. In production, a real secret store (Vault, AWS Secrets
Manager, GCP Secret Manager) is non-negotiable. For this project scope with
local-only DBs and no real PII, `.env` is fine.

**6. "What's the difference between `docker compose up` and `up -d`?"**
Without `-d`, Compose runs in the foreground and streams logs to your
terminal; Ctrl-C stops everything. With `-d` (detached), it backgrounds
the containers and returns immediately. Use `up` while developing,
`up -d` when you want the stack running but your terminal back.

**7. "Why isn't there a backend Dockerfile yet?"**
Phase 1 only requires the stack to come up — running the Go server is
still `go run ./cmd/vellum` from the host. Phase 7 (Polish) adds a backend
Dockerfile so the entire system can run inside Compose. That'd let
`docker compose up` truly bring up everything; today the human starts
the backend in another terminal.

**8. "What's `corepack` and why use it for pnpm?"**
`corepack` ships with Node 16+. It's a "package-manager manager" — you
ask for `pnpm` and corepack downloads + activates the version specified
in `package.json` (or latest). Saves the team from "wrong pnpm version"
bugs without a global install.

**9. "What does `internal/` enforce that just naming the directory
`private/` would not?"**
The Go *compiler* enforces it. Any package outside this module that
tries `import "github.com/kubeboiii/vellum/internal/pipeline"` will not
compile. Naming alone is convention; `internal/` is a contract.

**10. "Why not use Go's stdlib `net/http` instead of Gin?"**
You could — `net/http` works fine. Gin's value: middleware (rate-limit,
auth, logging) is much cleaner; JSON binding/validation is a one-liner;
routing supports path parameters out of the box. Cost: one more
dependency to defend in interviews. We picked it because the time it
saves on routes 2–10 outweighs the "extra dep" cost.

---

## 7. Things you should be able to do after Phase 1

Practice these end-to-end without the docs:

- [ ] Bring the stack up: `docker compose -f docker/compose.yaml up -d`
- [ ] Confirm all three containers are `healthy`: `docker compose ps`
- [ ] Connect to Postgres and verify the timescaledb extension is loaded.
- [ ] Tear the stack down keeping volumes: `down`. Versus nuking volumes:
      `down -v`.
- [ ] Boot the backend (`go run ./cmd/vellum`) and curl `/health`.
- [ ] Boot the frontend (`pnpm dev`) and open `localhost:3000`.
- [ ] Modify `docker/compose.yaml` to expose Redis on a non-default port
      and bring the stack back up. (Builds intuition.)
