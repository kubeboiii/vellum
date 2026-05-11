// Phase 5 — Typed fetch client for the IMS backend.
//
// Reads NEXT_PUBLIC_API_BASE at build time; defaults to localhost:8080
// for `pnpm dev`. The NEXT_PUBLIC_ prefix is how Next.js makes an env
// var available in the browser bundle (without it, the var is
// server-only).
//
// Every function wraps `fetch` and throws APIError on non-2xx so the
// page components can render error states uniformly (and try/catch
// only one error type).

import {
  APIError,
  type FieldError,
  type Health,
  type IncidentDetailResponse,
  type IncidentsListResponse,
  type RCA,
  type Signal,
  type SignalsPageResponse,
  type Status,
  type TransitionsResponse,
  type WorkItem,
} from "./types";

const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:8080";

// request is the workhorse: serialises the body, sets the right
// headers, raises APIError on non-2xx. Generic over the success
// response shape so callers don't cast.
async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init.headers,
    },
    // The 2s-polling live feed shouldn't read from the HTTP cache.
    // Next.js's default fetch is cached in Server Components; this
    // disables that for client-side polling fetches too.
    cache: "no-store",
  });
  if (!res.ok) {
    let message = `${res.status} ${res.statusText}`;
    let fields: FieldError[] | undefined;
    try {
      const body = await res.json();
      if (body?.error) message = body.error as string;
      if (Array.isArray(body?.fields)) fields = body.fields as FieldError[];
    } catch {
      // body wasn't JSON; fall through with the status text.
    }
    throw new APIError(message, res.status, fields);
  }
  // Some endpoints (PATCH /state, POST /rca) return JSON; other
  // requests might be 204. We `await res.json()` only when the
  // response has a body.
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

// ---- Read endpoints ----

export function listIncidents(): Promise<IncidentsListResponse> {
  return request<IncidentsListResponse>("/v1/incidents");
}

// listClosedIncidents powers the /incidents/closed history page.
// Limit caps at 500 server-side (see api.parseLimit).
export function listClosedIncidents(limit = 100): Promise<IncidentsListResponse> {
  return request<IncidentsListResponse>(`/v1/incidents/closed?limit=${limit}`);
}

export function listTransitions(id: string): Promise<TransitionsResponse> {
  return request<TransitionsResponse>(`/v1/incidents/${id}/transitions`);
}

export function getIncident(
  id: string,
): Promise<IncidentDetailResponse> {
  return request<IncidentDetailResponse>(`/v1/incidents/${id}`);
}

export function listSignals(
  id: string,
  page = 1,
  perPage = 50,
): Promise<SignalsPageResponse> {
  return request<SignalsPageResponse>(
    `/v1/incidents/${id}/signals?page=${page}&per_page=${perPage}`,
  );
}

// listSignalsBulk pulls up to `maxPages * perPage` signals across
// multiple paginated requests. Useful for SignalFrequency /
// PayloadFingerprints which need to see "most" signals on an
// incident, not just the latest 50. Stops early if the page count
// is exhausted.
export async function listSignalsBulk(
  id: string,
  maxPages = 3,
  perPage = 50,
): Promise<Signal[]> {
  const out: Signal[] = [];
  for (let p = 1; p <= maxPages; p++) {
    const page = await listSignals(id, p, perPage);
    out.push(...page.items);
    if (page.items.length < perPage) break;
    if (out.length >= page.total) break;
  }
  return out;
}

// getHealth pings /health for dependency status + queue depth.
// Throws APIError like the others; HealthStrip catches and renders
// a "backend offline" chip so the dashboard still works.
export function getHealth(): Promise<Health> {
  return request<Health>("/health");
}

// postSignal fires a single synthetic signal at the backend's
// ingestion endpoint. Used by /load-test and /simulate; not part
// of the steady-state read flow. Returns parsed body OR throws
// APIError on non-2xx (e.g. 503 ResourceExhausted).
export interface SignalInput {
  signal_id?: string;
  component_id: string;
  component_type: string;
  severity: "P0" | "P1" | "P2" | "P3";
  timestamp?: string;
  source: string;
  payload: Record<string, unknown>;
}
export function postSignal(s: SignalInput): Promise<unknown> {
  return request<unknown>("/v1/signals", {
    method: "POST",
    body: JSON.stringify({
      ...s,
      timestamp: s.timestamp ?? new Date().toISOString(),
    }),
  });
}

// ---- Write endpoints ----

export function patchState(
  id: string,
  to: Status,
  actor?: string,
  reason?: string,
): Promise<{ work_item: WorkItem }> {
  return request<{ work_item: WorkItem }>(
    `/v1/incidents/${id}/state`,
    {
      method: "PATCH",
      body: JSON.stringify({ to, actor: actor ?? "dashboard", reason: reason ?? "" }),
    },
  );
}

export interface PostRCABody {
  root_cause_category: string;
  fix_applied: string;
  prevention_steps: string;
  submitted_by: string;
}

export function postRCA(
  id: string,
  body: PostRCABody,
): Promise<{ work_item: WorkItem; rca: RCA }> {
  return request<{ work_item: WorkItem; rca: RCA }>(
    `/v1/incidents/${id}/rca`,
    {
      method: "POST",
      body: JSON.stringify(body),
    },
  );
}
