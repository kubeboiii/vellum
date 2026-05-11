"use client";

import Link from "next/link";

import type { WorkItem } from "@/lib/types";

interface Props {
  items: WorkItem[];
}

interface Offender {
  component_id: string;
  count: number;
  avgMttrSec: number;

  latestId: string;
}

export function RepeatOffenders({ items }: Props) {

  const byComp = new Map<string, { items: WorkItem[]; latest: WorkItem }>();
  for (const wi of items) {
    if (typeof wi.mttr_seconds !== "number") continue;
    const existing = byComp.get(wi.component_id);
    if (existing) {
      existing.items.push(wi);
      if (
        wi.closed_at &&
        existing.latest.closed_at &&
        new Date(wi.closed_at) > new Date(existing.latest.closed_at)
      ) {
        existing.latest = wi;
      }
    } else {
      byComp.set(wi.component_id, { items: [wi], latest: wi });
    }
  }

  const offenders: Offender[] = Array.from(byComp.entries())
    .map(([component_id, g]) => {
      const sum = g.items.reduce((a, x) => a + (x.mttr_seconds ?? 0), 0);
      return {
        component_id,
        count: g.items.length,
        avgMttrSec: sum / g.items.length,
        latestId: g.latest.id,
      };
    })
    .sort((a, b) => b.count - a.count)
    .slice(0, 5);

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Repeat offenders
        </h3>
        <span className="font-mono text-meta text-text-tertiary">top {offenders.length}</span>
      </header>
      {offenders.length === 0 ? (
        <p className="font-mono text-meta text-text-tertiary">
          No repeat-closure components yet.
        </p>
      ) : (
        <ul className="space-y-2">
          {offenders.map((o) => (
            <li key={o.component_id}>
              <Link
                href={`/incidents/${o.latestId}`}
                className="group flex items-center gap-3 focus-visible:outline-none"
              >
                <span className="truncate font-mono text-data text-text-primary group-hover:text-accent">
                  {o.component_id}
                </span>
                <span className="ml-auto inline-flex shrink-0 items-center gap-2 font-mono text-meta tabular-nums">
                  <span className="rounded-sm bg-sev-p0-bg px-1.5 py-0.5 text-sev-p0">
                    {o.count}× closed
                  </span>
                  <span className="text-text-secondary">
                    avg {fmtMTTR(o.avgMttrSec)}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function fmtMTTR(s: number): string {
  if (s < 60) return `${Math.round(s)}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}
