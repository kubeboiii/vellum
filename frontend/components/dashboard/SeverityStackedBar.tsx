"use client";

import type { Severity, WorkItem } from "@/lib/types";

const ORDER: Severity[] = ["P0", "P1", "P2", "P3"];

const FILL: Record<Severity, string> = {
  P0: "bg-sev-p0",
  P1: "bg-sev-p1",
  P2: "bg-sev-p2",
  P3: "bg-sev-p3",
};

const TEXT: Record<Severity, string> = {
  P0: "text-sev-p0",
  P1: "text-sev-p1",
  P2: "text-sev-p2",
  P3: "text-sev-p3",
};

interface Props {
  items: WorkItem[];
}

export function SeverityStackedBar({ items }: Props) {
  const counts: Record<Severity, number> = { P0: 0, P1: 0, P2: 0, P3: 0 };
  for (const i of items) counts[i.severity] += 1;
  const total = items.length;

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
            aria-hidden
          />
          <h2 className="font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
            Severity mix
          </h2>
        </div>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {total} active
        </span>
      </header>

      {}
      <div className="mb-3 flex flex-wrap items-center gap-4">
        {ORDER.map((sev) => {
          const pulse = sev === "P0" && counts.P0 > 0;
          return (
            <span
              key={sev}
              className="inline-flex items-center gap-1.5 font-mono text-meta"
              title={`${sev} · ${counts[sev]} active`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${FILL[sev]} ${pulse ? "animate-pulse-live" : ""}`}
                aria-hidden
              />
              <span className={`uppercase tracking-[0.05em] ${TEXT[sev]}`}>
                {sev}
              </span>
              <span className="text-text-secondary tabular-nums">
                {counts[sev]}
              </span>
            </span>
          );
        })}
      </div>

      {}
      <div
        className="flex h-1.5 w-full overflow-hidden rounded-sm bg-bg-elevated"
        role="img"
        aria-label="active incidents by severity"
      >
        {total === 0 ? (
          <div className="h-full w-full" aria-label="no active incidents" />
        ) : (
          ORDER.map((sev) => {
            const w = (counts[sev] / total) * 100;
            if (w === 0) return null;
            return (
              <div
                key={sev}
                className={`h-full ${FILL[sev]}`}
                style={{ width: `${w}%` }}
                title={`${sev} · ${counts[sev]} (${w.toFixed(0)}%)`}
              />
            );
          })
        )}
      </div>
      {total === 0 && (
        <p className="mt-3 font-mono text-meta text-text-tertiary">
          No active incidents.
        </p>
      )}
    </section>
  );
}
