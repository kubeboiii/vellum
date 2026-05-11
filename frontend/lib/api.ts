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

    }
    throw new APIError(message, res.status, fields);
  }

  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export function listIncidents(): Promise<IncidentsListResponse> {
  return request<IncidentsListResponse>("/v1/incidents");
}

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

export function getHealth(): Promise<Health> {
  return request<Health>("/health");
}

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
