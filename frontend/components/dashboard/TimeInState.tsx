// TimeInState — from the state_transitions audit log, compute how
// long the incident spent in each state. Helps the post-mortem
// author articulate "we lost 12 minutes between OPEN and someone
// taking it" without re-deriving from timestamps by hand.

"use client";

import { useEffect, useState } from "react";

import type { Status, StateTransition, WorkItem } from "@/lib/types";

const ORDER: Status[] = ["OPEN", "INVESTIGATING", "RESOLVED", "CLOSED"];

const FILL: Record<Status, string> = {
  OPEN: "bg-sev-p0",
  INVESTIGATING: "bg-sev-p1",
  RESOLVED: "bg-accent",
  CLOSED: "bg-text-tertiary",
};

const TEXT: Record<Status, string> = {
  OPEN: "text-sev-p0",
  INVESTIGATING: "text-sev-p1",
  RESOLVED: "text-accent",
  CLOSED: "text-text-tertiary",
};

interface Props {
  work_item: WorkItem;
  transitions: StateTransition[];
}

export function TimeInState({ work_item, transitions }: Props) {
  // Same "right edge ticks while open" trick as TransitionTimeline.
  const [nowMs, setNowMs] = useState<number | null>(null);
  useEffect(() => {
    setNowMs(Date.now());
    const id = setInterval(() => setNowMs(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  // Build a synthetic transition list including OPEN @ start, then
  // walk pairs to derive durations per state. Final segment uses
  // closed_at if available, else now.
  const sorted = [...transitions].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  );
  const start = new Date(work_item.first_signal_ts).getTime();
  const points: Array<{ state: Status; t: number }> = [
    { state: "OPEN", t: start },
  ];
  for (const tr of sorted) {
    points.push({ state: tr.to_state, t: new Date(tr.created_at).getTime() });
  }
  const tail = work_item.closed_at
    ? new Date(work_item.closed_at).getTime()
    : (nowMs ?? new Date(work_item.last_signal_ts).getTime());

  const durations: Record<Status, number> = {
    OPEN: 0,
    INVESTIGATING: 0,
    RESOLVED: 0,
    CLOSED: 0,
  };
  for (let i = 0; i < points.length; i++) {
    const cur = points[i];
    const next = points[i + 1]?.t ?? tail;
    durations[cur.state] += Math.max(0, next - cur.t);
  }
  const total = Object.values(durations).reduce((a, b) => a + b, 0);

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Time in each state
        </h3>
        <span className="font-mono text-meta text-text-tertiary">
          {fmtDur(total)} total
        </span>
      </header>
      <ul className="space-y-2">
        {ORDER.map((s) => {
          const d = durations[s];
          const pct = total > 0 ? (d / total) * 100 : 0;
          const active = work_item.status === s;
          return (
            <li key={s} className="flex items-center gap-3">
              <span
                className={`w-28 shrink-0 font-mono text-meta uppercase tracking-[0.05em] ${TEXT[s]} ${active ? "" : "opacity-80"}`}
              >
                {s.toLowerCase()}
                {active && (
                  <span
                    className={`ml-1 inline-block h-1 w-1 animate-pulse-live rounded-full align-middle ${FILL[s]}`}
                    aria-hidden
                  />
                )}
              </span>
              <div className="relative h-1.5 flex-1 overflow-hidden rounded-sm bg-bg-elevated">
                <div
                  className={`h-full ${FILL[s]}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
              <span className="w-24 shrink-0 text-right font-mono text-meta tabular-nums text-text-secondary">
                {fmtDur(d)}{" "}
                <span className="text-text-tertiary">
                  · {pct.toFixed(0)}%
                </span>
              </span>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function fmtDur(ms: number): string {
  if (ms === 0) return "—";
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}
