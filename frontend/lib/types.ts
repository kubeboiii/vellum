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

export interface WorkItem {
  id: string;
  component_id: string;
  component_type: ComponentType;
  severity: Severity;
  status: Status;
  signal_count: number;
  first_signal_ts: string;
  last_signal_ts: string;
  mttr_seconds?: number;
  incident_start?: string;
  incident_end?: string;
  closed_at?: string;
  created_at: string;
  updated_at: string;
}

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

export interface FieldError {
  field: string;
  error: string;
}

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

  dependencies: Partial<Record<"postgres" | "mongo" | "redis" | "timescale", Dependency>>;

  counters?: Record<string, number>;
}

export class APIError extends Error {
  status: number;
  fields?: FieldError[];
  constructor(message: string, status: number, fields?: FieldError[]) {
    super(message);
    this.status = status;
    this.fields = fields;
  }
}
