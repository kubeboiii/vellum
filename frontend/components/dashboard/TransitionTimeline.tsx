"use client";

import { useEffect, useState } from "react";

import type { Status, StateTransition, WorkItem } from "@/lib/types";

const STATE_FILL: Record<Status, string> = {
  OPEN: "bg-sev-p0",
  INVESTIGATING: "bg-sev-p1",
  RESOLVED: "bg-accent",
  CLOSED: "bg-text-tertiary",
};

interface Props {
  work_item: WorkItem;
  transitions: StateTransition[];
}

export function TransitionTimeline({ work_item, transitions }: Props) {

  const [nowMs, setNowMs] = useState<number | null>(null);
  useEffect(() => {
    setNowMs(Date.now());
    const id = setInterval(() => setNowMs(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  const start = new Date(work_item.first_signal_ts).getTime();
  const end = work_item.closed_at
    ? new Date(work_item.closed_at).getTime()
    : (nowMs ?? new Date(work_item.last_signal_ts).getTime());
  const span = Math.max(1, end - start);
  const spanLabel = fmtSpan(span);

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Transition timeline
        </h3>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {spanLabel} since first signal
        </span>
      </header>
      <div className="relative h-7 w-full overflow-visible">
        {}
        <div className="absolute inset-x-0 top-1/2 h-1 -translate-y-1/2 rounded-sm bg-bg-elevated" />
        {}
        <Tick
          label={`OPEN @ start`}
          state={work_item.status === "OPEN" ? "OPEN" : "OPEN"}
          left={0}
        />
        {transitions.map((t) => {
          const ts = new Date(t.created_at).getTime();
          const left = Math.max(0, Math.min(100, ((ts - start) / span) * 100));
          return (
            <Tick
              key={t.id}
              label={`${t.from_state} → ${t.to_state}${t.actor ? ` · ${t.actor}` : ""}${t.reason ? ` · ${t.reason}` : ""} · ${fmtAbsTime(t.created_at)}`}
              state={t.to_state}
              left={left}
            />
          );
        })}
      </div>
      {transitions.length === 0 && (
        <p className="mt-2 font-mono text-meta text-text-tertiary">
          No transitions yet. Incident is still {work_item.status}.
        </p>
      )}
    </section>
  );
}

function Tick({
  label,
  state,
  left,
}: {
  label: string;
  state: Status;
  left: number;
}) {
  return (
    <div
      className="group absolute top-1/2 -translate-x-1/2 -translate-y-1/2"
      style={{ left: `${left}%` }}
    >
      <div
        className={`h-3 w-[3px] ${STATE_FILL[state]} group-hover:h-4`}
        title={label}
      />
      <span className="pointer-events-none absolute left-1/2 top-5 hidden -translate-x-1/2 whitespace-nowrap rounded-sm border border-border-subtle bg-bg-surface px-1.5 py-0.5 font-mono text-meta text-text-secondary group-hover:block">
        {label}
      </span>
    </div>
  );
}

function fmtSpan(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

function fmtAbsTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], { hour12: false });
}
