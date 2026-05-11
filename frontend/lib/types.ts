// Phase 5 — TypeScript mirrors of backend models.
//
// Hand-written rather than generated. The shapes are small enough that
// keeping them in sync by hand is cheaper than wiring openapi-typescript
// or a similar codegen tool. Every field name here MUST match the JSON
// tag in the corresponding Go struct (backend/internal/model/*.go).

export type Severity = "P0" | "P1" | "P2" | "P3";

export type Status = "OPEN" | "INVESTIGATING" | "RESOLVED" | "CLOSED";

export type ComponentType =
  | "API"
  | "MCP_HOST"
  | "CACHE"
  | "QUEUE"
  | "RDBMS"
  | "NOSQL"
  | "OTHER";

export type RootCauseCategory =
  | "CODE_DEFECT"
  | "INFRASTRUCTURE"
  | "CONFIG_CHANGE"
  | "EXTERNAL_DEPENDENCY"
  | "CAPACITY"
  | "HUMAN_ERROR"
  | "OTHER";

// WorkItem mirrors backend/internal/model/work_item.go.
// Nullable fields are pointers in Go (omitempty), so we mark them
// optional here.
export interface WorkItem {
  id: string;
  component_id: string;
  component_type: ComponentType;
  severity: Severity;
  status: Status;
  signal_count: number;
  first_signal_ts: string; // ISO-8601
  last_signal_ts: string;
  mttr_seconds?: number;
  incident_start?: string;
  incident_end?: string;
  closed_at?: string;
  created_at: string;
  updated_at: string;
}

// RCA mirrors backend/internal/model/rca.go.
export interface RCA {
  id: string;
  work_item_id: string;
  incident_start: string;
  incident_end: string;
  root_cause_category: RootCauseCategory;
  fix_applied: string;
  prevention_steps: string;
  submitted_by: string;
  created_at: string;
}

// Signal is the raw audit-log shape returned by /v1/incidents/:id/signals.
// payload is free-form; we type it as `unknown` and the UI just
// JSON.stringifies it.
export interface Signal {
  signal_id: string;
  work_item_id: string;
  component_id: string;
  component_type: ComponentType;
  severity: Severity;
  timestamp: string;
  source: string;
  payload: unknown;
  ingested_at: string;
}

export interface IncidentsListResponse {
  items: WorkItem[];
  count: number;
}

export interface IncidentDetailResponse {
  work_item: WorkItem;
  rca?: RCA;
}

export interface SignalsPageResponse {
  items: Signal[];
  page: number;
  per_page: number;
  total: number;
}

// StateTransition mirrors backend/internal/model/state_transition.go.
// Reason and actor are optional ("omitempty" on the Go side); JSON
// won't include them when empty.
export interface StateTransition {
  id: string;
  work_item_id: string;
  from_state: Status;
  to_state: Status;
  reason?: string;
  actor?: string;
  created_at: string;
}

export interface TransitionsResponse {
  items: StateTransition[];
  count: number;
}

// FieldError matches model.FieldError. Returned in the body of 422
// responses to POST /rca when individual fields fail validation.
export interface FieldError {
  field: string;
  error: string;
}

// Health mirrors the JSON returned by GET /health (FR-8.1).
//
// The backend uses "up"/"down"/"degraded" on each dep and
// "healthy"/"degraded"/"down" at the roll-up — we accept all of
// these plus the spec'd "ok" alias so the UI tolerates either.
// `timescale` is optional because v1 backend omits it (Timescale
// is a Postgres extension; the same conn is reused).
export type DepStatus = "ok" | "up" | "degraded" | "down" | "healthy";
export interface Dependency {
  status: DepStatus;
  latency_ms: number;
}
export interface Health {
  status: DepStatus;
  uptime_seconds: number;
  queue_depth: number;
  queue_capacity: number;
  // Partial<> because the backend may omit any dep; HealthStrip
  // iterates the keys that exist instead of indexing fixed ones.
  dependencies: Partial<Record<"postgres" | "mongo" | "redis" | "timescale", Dependency>>;
  // The backend additionally exposes counters; we don't use them
  // in v1 of HealthStrip but keep the field optional for forwards
  // compatibility.
  counters?: Record<string, number>;
}

// APIError is the shape we throw from the client on non-2xx.
// `fields` is only populated for 422 from POST /rca.
export class APIError extends Error {
  status: number;
  fields?: FieldError[];
  constructor(message: string, status: number, fields?: FieldError[]) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}
