// THEME.md §7.2 — Incident detail page (two-column).
//
// Left (3/5): summary card, RCA panel (if CLOSED), state transitions.
// Right (2/5): scrollable signal list with expandable JSON payloads.
//
// All Client-side — PATCH /state needs onClick + state-machine
// state across renders.

"use client";

import { IconChevronLeft, IconChevronRight } from "@tabler/icons-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { Button } from "@/components/Button";
import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { PayloadFingerprints } from "@/components/dashboard/PayloadFingerprints";
import { SignalFrequency } from "@/components/dashboard/SignalFrequency";
import { TimeInState } from "@/components/dashboard/TimeInState";
import { TransitionTimeline } from "@/components/dashboard/TransitionTimeline";
import { Nav } from "@/components/Nav";
import { PayloadJSON } from "@/components/PayloadJSON";
import { SeverityBadge } from "@/components/SeverityBadge";
import { StatePill } from "@/components/StatePill";
import { Timeline } from "@/components/Timeline";
import { useToast } from "@/components/Toast";
import { APIError } from "@/lib/types";
import {
  getIncident,
  listSignals,
  listSignalsBulk,
  listTransitions,
  patchState,
} from "@/lib/api";
import { fingerprint } from "@/lib/fingerprint";
import type {
  IncidentDetailResponse,
  Signal,
  SignalsPageResponse,
  Status,
  StateTransition,
} from "@/lib/types";

// nextLegalStates mirrors the State pattern's CanTransitionTo rules.
// CLOSED is reached only via POST /rca, so it's intentionally absent
// from RESOLVED's list — the page shows a "Submit RCA →" button
// instead.
const nextLegalStates: Record<Status, Status[]> = {
  OPEN: ["INVESTIGATING"],
  INVESTIGATING: ["RESOLVED"],
  RESOLVED: [],
  CLOSED: [],
};

export default function IncidentDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;

  const [data, setData] = useState<IncidentDetailResponse | null>(null);
  const [signals, setSignals] = useState<SignalsPageResponse | null>(null);
  // bulkSignals is a separate buffer used by the histogram &
  // fingerprint widgets — they need a wider window than the
  // paginated `signals` list shows. Fetched lazily to avoid bloat
  // on incidents with thousands of signals.
  const [bulkSignals, setBulkSignals] = useState<Signal[] | null>(null);
  const [fpFilter, setFpFilter] = useState<string | null>(null);
  const [transitions, setTransitions] = useState<StateTransition[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  const reload = useCallback(async () => {
    try {
      const [d, s, t] = await Promise.all([
        getIncident(id),
        listSignals(id, 1, 50),
        listTransitions(id),
      ]);
      setData(d);
      setSignals(s);
      setTransitions(t.items);
      setError(null);
    } catch (e) {
      setError((e as Error).message);
    }
  }, [id]);

  // Fetch up to 150 signals once on first mount for the widgets.
  // We don't refetch on state advances — the timeline data shape
  // doesn't change just because someone clicked "advance".
  useEffect(() => {
    let cancelled = false;
    listSignalsBulk(id, 3, 50)
      .then((sigs) => {
        if (!cancelled) setBulkSignals(sigs);
      })
      .catch(() => {
        // If the bulk fetch fails, the regular `signals` page
        // already covers the raw-signal list; widgets just hide.
      });
    return () => {
      cancelled = true;
    };
  }, [id]);

  useEffect(() => {
    reload();
  }, [reload]);

  const advance = async (to: Status) => {
    setBusy(true);
    try {
      await patchState(id, to, "dashboard", "advanced via dashboard");
      await reload();
      toast.push("success", `→ ${to}`);
    } catch (e) {
      // The 409 / 422 / 500 distinction is in APIError.status; toast
      // surfaces it as the message so the user can read what went
      // wrong without opening devtools.
      const msg =
        e instanceof APIError ? `Transition rejected (${e.status}): ${e.message}` : (e as Error).message;
      setError(msg);
      toast.push("error", msg);
    } finally {
      setBusy(false);
    }
  };

  if (error && !data) {
    return (
      <div className="min-h-screen bg-bg-base text-text-primary">
        <Nav title="Incident" />
        <HealthStrip />
        <main className="mx-auto max-w-6xl px-6 py-6">
          <BackLink />
          <div className="mt-4 rounded-md border border-sev-p0-border bg-sev-p0-bg/40 px-4 py-3 font-mono text-meta text-red-300">
            {error}
          </div>
        </main>
      </div>
    );
  }
  if (!data) {
    return (
      <div className="min-h-screen bg-bg-base text-text-primary">
        <Nav title="Incident" />
        <HealthStrip />
        <main className="mx-auto max-w-6xl px-6 py-6">
          <div className="inline-flex items-center gap-2 font-mono text-meta text-text-tertiary">
            <span
              className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
              aria-hidden
            />
            Loading…
          </div>
        </main>
      </div>
    );
  }

  const wi = data.work_item;
  const nextStates = nextLegalStates[wi.status];

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Incident" />
      <main className="mx-auto max-w-[1400px] px-6 py-4 space-y-4">
        <BackLink />

        {error && (
          <div className="rounded-sm border border-sev-p0-border bg-sev-p0-bg/40 px-3 py-2 font-mono text-meta text-red-300">
            {error}
          </div>
        )}

        <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
          {/* ───────────── LEFT 3/5 ───────────── */}
          <div className="space-y-4 lg:col-span-3">
            {/* Summary card — lime hover glow to match landing-page
                card pattern (StatCard, Capabilities). */}
            <section
              className="rounded-md border border-border-subtle bg-bg-surface p-4 transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
              onMouseEnter={(e) => {
                e.currentTarget.style.boxShadow =
                  "0 0 28px -10px rgba(190,242,100,0.35)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = "";
              }}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <h1 className="truncate font-mono text-page font-medium text-text-primary">
                    {wi.component_id}
                  </h1>
                  <div className="mt-1 flex items-center gap-2 font-mono text-meta text-text-tertiary">
                    <span>{wi.component_type}</span>
                    <span aria-hidden>·</span>
                    <span>{wi.id.slice(0, 8)}</span>
                  </div>
                </div>
                <div className="flex flex-col items-end gap-2">
                  <SeverityBadge severity={wi.severity} />
                  <StatePill
                    status={wi.status}
                    pulseDot={wi.severity === "P0" && wi.status === "OPEN"}
                  />
                </div>
              </div>

              <dl className="mt-4 grid grid-cols-2 gap-x-6 gap-y-3 border-t border-border-subtle pt-4">
                <Field label="Signals" value={wi.signal_count.toLocaleString()} mono />
                <Field label="First signal" value={fmtTime(wi.first_signal_ts)} mono />
                <Field label="Last signal" value={fmtTime(wi.last_signal_ts)} mono />
                {wi.mttr_seconds != null && (
                  <Field label="MTTR" value={fmtDuration(wi.mttr_seconds)} mono />
                )}
                {wi.closed_at && (
                  <Field label="Closed at" value={fmtTime(wi.closed_at)} mono />
                )}
              </dl>

              {/* Action row */}
              <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-border-subtle pt-3">
                {nextStates.map((to) => (
                  <Button
                    key={to}
                    variant="ghost"
                    disabled={busy}
                    onClick={() => advance(to)}
                  >
                    Advance to {to}
                  </Button>
                ))}
                {wi.status === "RESOLVED" && (
                  <Link href={`/incidents/${wi.id}/rca`}>
                    <Button variant="primary">Submit RCA & Close</Button>
                  </Link>
                )}
                {wi.status === "CLOSED" && (
                  <span className="font-mono text-meta text-text-tertiary">
                    Incident closed
                  </span>
                )}
              </div>
            </section>

            {/* Horizontal transition timeline — the "at a glance"
                view. Vertical Timeline below has the detail. */}
            <TransitionTimeline work_item={wi} transitions={transitions} />

            {/* Time-in-each-state breakdown. Surfaces "where the
                time actually went" for post-mortem authors. */}
            <TimeInState work_item={wi} transitions={transitions} />

            {/* Signal-shape & frequency analysis. Hidden until the
                bulk fetch lands. */}
            {bulkSignals && bulkSignals.length > 0 && (
              <>
                <SignalFrequency signals={bulkSignals} />
                <PayloadFingerprints
                  signals={bulkSignals}
                  selected={fpFilter}
                  onSelect={setFpFilter}
                />
              </>
            )}

            {/* Timeline panel — state-transition audit log
                (FR-7.2). Always rendered; empty state for OPEN
                incidents that haven't transitioned yet. */}
            <section
              className="overflow-hidden rounded-md border border-border-subtle bg-bg-surface transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
              onMouseEnter={(e) => {
                e.currentTarget.style.boxShadow =
                  "0 0 28px -10px rgba(190,242,100,0.30)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = "";
              }}
            >
              <header className="border-b border-border-subtle bg-bg-elevated px-4 py-2">
                <h2 className="relative font-sans text-card font-semibold text-text-primary">
                  Timeline · audit log
                  <span className="absolute -bottom-1 left-0 h-px w-6 bg-accent" aria-hidden />
                </h2>
              </header>
              <Timeline
                transitions={transitions}
                empty="No state transitions yet — incident is in its initial state."
              />
            </section>

            {/* RCA panel (if present) */}
            {data.rca && (
              <section
                className="rounded-md border border-border-subtle bg-bg-surface p-4 transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
                onMouseEnter={(e) => {
                  e.currentTarget.style.boxShadow =
                    "0 0 28px -10px rgba(190,242,100,0.35)";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.boxShadow = "";
                }}
              >
                <h2 className="relative pb-2 font-sans text-section font-semibold text-text-primary">
                  Root Cause Analysis
                  <span className="absolute bottom-0 left-0 h-px w-6 bg-accent" aria-hidden />
                </h2>
                <dl className="space-y-3 pt-2">
                  <Field label="Category" value={data.rca.root_cause_category} mono />
                  <Field
                    label="Fix applied"
                    value={data.rca.fix_applied}
                    wrap
                  />
                  <Field
                    label="Prevention"
                    value={data.rca.prevention_steps}
                    wrap
                  />
                  <Field label="Submitted by" value={data.rca.submitted_by} mono />
                </dl>
              </section>
            )}
          </div>

          {/* ───────────── RIGHT 2/5 ───────────── */}
          <div className="space-y-4 lg:col-span-2">
            <section
              className="overflow-hidden rounded-md border border-border-subtle bg-bg-surface transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
              onMouseEnter={(e) => {
                e.currentTarget.style.boxShadow =
                  "0 0 28px -10px rgba(190,242,100,0.30)";
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.boxShadow = "";
              }}
            >
              <header className="flex items-center justify-between border-b border-border-subtle bg-bg-elevated px-4 py-2">
                <h2 className="font-sans text-card font-semibold text-text-primary">
                  Raw Signals
                </h2>
                <span className="font-mono text-meta text-text-tertiary">
                  {signals ? `${signals.total.toLocaleString()} total` : "…"}
                </span>
              </header>
              <div className="max-h-[640px] overflow-y-auto">
                {!signals && (
                  <div className="flex items-center gap-2 px-4 py-6 font-mono text-meta text-text-tertiary">
                    <span
                      className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
                      aria-hidden
                    />
                    Loading signals…
                  </div>
                )}
                {signals && signals.items.length === 0 && (
                  <div className="px-4 py-6 font-mono text-meta text-text-tertiary">
                    No signals attached.
                  </div>
                )}
                {signals?.items
                  .filter((s) => fpFilter == null || fingerprint(s.payload) === fpFilter)
                  .map((s) => (
                    <SignalEntry key={s.signal_id} signal={s} />
                  ))}
              </div>
            </section>
          </div>
        </div>
      </main>
    </div>
  );
}

// ---- helpers / sub-components ----

function BackLink() {
  return (
    <Link
      href="/dashboard"
      className="inline-flex items-center gap-1 font-mono text-meta text-text-secondary transition-colors duration-fast hover:text-text-primary"
    >
      <IconChevronLeft size={14} />
      live feed
    </Link>
  );
}

function Field({
  label,
  value,
  mono,
  wrap,
}: {
  label: string;
  value: string;
  mono?: boolean;
  wrap?: boolean;
}) {
  return (
    <div>
      <dt className="font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
        {label}
      </dt>
      <dd
        className={`${mono ? "font-mono text-data" : "font-sans text-body"} ${
          wrap ? "whitespace-pre-wrap" : ""
        } text-text-primary`}
      >
        {value}
      </dd>
    </div>
  );
}

function SignalEntry({ signal }: { signal: Signal }) {
  const [open, setOpen] = useState(false);
  return (
    <button
      type="button"
      onClick={() => setOpen((o) => !o)}
      className="block w-full border-b border-border-subtle text-left transition-colors duration-fast hover:bg-bg-elevated focus-visible:bg-bg-elevated focus-visible:outline-none"
    >
      <div className="grid grid-cols-[14px_1fr_auto] items-center gap-2 px-4 py-2 font-mono text-meta">
        <IconChevronRight
          size={12}
          className={`text-text-tertiary transition-transform duration-fast ${open ? "rotate-90" : ""}`}
          aria-hidden
        />
        <span className="truncate text-text-secondary">
          <SeverityBadge severity={signal.severity} />{" "}
          <span className="ml-2 text-text-tertiary">{signal.source}</span>{" "}
          <span className="ml-1 text-text-tertiary">
            {signal.signal_id.slice(0, 8)}
          </span>
        </span>
        <span className="text-text-tertiary">{fmtTime(signal.timestamp)}</span>
      </div>
      {open && (
        <div className="border-t border-border-subtle bg-bg-input px-4 py-3">
          <PayloadJSON value={signal.payload} />
        </div>
      )}
    </button>
  );
}

function fmtTime(iso: string): string {
  return new Date(iso).toLocaleString();
}

function fmtDuration(s: number): string {
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}
