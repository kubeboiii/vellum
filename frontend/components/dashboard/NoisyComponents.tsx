"use client";

import Link from "next/link";

import type { Severity, WorkItem } from "@/lib/types";

const FILL: Record<Severity, string> = {
  P0: "bg-sev-p0",
  P1: "bg-sev-p1",
  P2: "bg-sev-p2",
  P3: "bg-sev-p3",
};

interface Props {
  items: WorkItem[];
}

export function NoisyComponents({ items }: Props) {

  const top = [...items]
    .sort((a, b) => b.signal_count - a.signal_count)
    .slice(0, 5);
  const max = top[0]?.signal_count ?? 1;

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <h2 className="font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
          Noisiest components
        </h2>
        <span className="font-mono text-meta text-text-tertiary">
          top {top.length}
        </span>
      </header>
      {top.length === 0 ? (
        <p className="font-mono text-meta text-text-tertiary">
          Nothing screaming right now.
        </p>
      ) : (
        <ul className="space-y-2">
          {top.map((wi) => {
            const pct = Math.max(4, (wi.signal_count / max) * 100);
            return (
              <li key={wi.id}>
                <Link
                  href={`/incidents/${wi.id}`}
                  className="group block focus-visible:outline-none"
                >
                  <div className="mb-0.5 flex items-baseline justify-between gap-3">
                    <span className="truncate font-mono text-data text-text-primary group-hover:text-accent">
                      {wi.component_id}
                    </span>
                    <span className="shrink-0 font-mono text-meta tabular-nums text-text-secondary">
                      {wi.signal_count.toLocaleString()}{" "}
                      <span className="text-text-tertiary">signals</span>
                    </span>
                  </div>
                  <div className="relative h-1 w-full overflow-hidden rounded-sm bg-bg-elevated">
                    <div
                      className={`h-full ${FILL[wi.severity]} transition-[width] duration-base ease-out`}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                </Link>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
