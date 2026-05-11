// CategoryBreakdown — distribution of closed incidents by RCA
// root_cause_category. Tier-1 constraint: the /v1/incidents/closed
// endpoint doesn't include the RCA, so we fetch RCAs lazily when
// the user clicks "Compute". This avoids 100 round-trips on page
// load while still surfacing the data on demand.

"use client";

import { useState } from "react";
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";

import { getIncident } from "@/lib/api";
import type { RootCauseCategory, WorkItem } from "@/lib/types";

interface Props {
  items: WorkItem[];
}

const ORDER: RootCauseCategory[] = [
  "CODE_DEFECT",
  "INFRASTRUCTURE",
  "CONFIG_CHANGE",
  "EXTERNAL_DEPENDENCY",
  "CAPACITY",
  "HUMAN_ERROR",
  "OTHER",
];

// Each category gets a distinct hue so the chart reads as
// categorical, not severity-ordered. All from the existing palette
// (severity colors + accent + annotation violet).
const FILL: Record<RootCauseCategory, string> = {
  CODE_DEFECT: "bg-sev-p0",
  INFRASTRUCTURE: "bg-sev-p1",
  CONFIG_CHANGE: "bg-accent",
  EXTERNAL_DEPENDENCY: "bg-annotation",
  CAPACITY: "bg-sev-p2",
  HUMAN_ERROR: "bg-sev-p3",
  OTHER: "bg-text-tertiary",
};

// Recharts can't read Tailwind class tokens — it needs raw colors
// for `fill`. Mirror the same palette by hex so the donut and the
// legend dots agree.
const FILL_HEX: Record<RootCauseCategory, string> = {
  CODE_DEFECT: "#EF4444",
  INFRASTRUCTURE: "#F59E0B",
  CONFIG_CHANGE: "#BEF264",
  EXTERNAL_DEPENDENCY: "#A78BFA",
  CAPACITY: "#84CC16",
  HUMAN_ERROR: "#A1A1AA",
  OTHER: "#52525B",
};

export function CategoryBreakdown({ items }: Props) {
  const [counts, setCounts] = useState<Record<RootCauseCategory, number> | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const compute = async () => {
    setBusy(true);
    setError(null);
    try {
      // Parallel fetch, capped at 50 for safety (page already caps
      // closed list at 100 in current API client).
      const sample = items.slice(0, 50);
      const results = await Promise.all(
        sample.map((wi) =>
          getIncident(wi.id).then((d) => d.rca?.root_cause_category).catch(() => undefined),
        ),
      );
      const next: Record<RootCauseCategory, number> = {
        CODE_DEFECT: 0,
        INFRASTRUCTURE: 0,
        CONFIG_CHANGE: 0,
        EXTERNAL_DEPENDENCY: 0,
        CAPACITY: 0,
        HUMAN_ERROR: 0,
        OTHER: 0,
      };
      for (const cat of results) {
        if (cat) next[cat] += 1;
      }
      setCounts(next);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const total = counts
    ? Object.values(counts).reduce((a, b) => a + b, 0)
    : 0;

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between gap-3">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Root cause categories
        </h3>
        {!counts && (
          <button
            type="button"
            onClick={compute}
            disabled={busy || items.length === 0}
            className="rounded-sm border border-border-subtle bg-transparent px-2 py-0.5 font-mono text-meta uppercase tracking-[0.05em] text-text-primary transition-colors duration-fast hover:bg-bg-elevated hover:border-border-strong disabled:opacity-50"
          >
            {busy ? "computing…" : "compute"}
          </button>
        )}
        {counts && (
          <span className="font-mono text-meta text-text-tertiary tabular-nums">
            {total} sampled
          </span>
        )}
      </header>

      {!counts && !error && (
        <p className="font-mono text-meta text-text-tertiary">
          Click compute to fetch each closed incident&apos;s RCA and
          tally categories. Sample capped at 50.
        </p>
      )}
      {error && (
        <p className="font-mono text-meta text-sev-p0">{error}</p>
      )}
      {counts && total > 0 && (
        <div className="grid grid-cols-1 items-center gap-3 sm:grid-cols-[160px_1fr]">
          {/* Donut. The hole in the middle holds the dominant
              category's share so the chart answers "which root
              cause is hurting us most?" at a single glance. */}
          <div className="relative mx-auto h-[140px] w-[140px]">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={ORDER.filter((c) => counts[c] > 0).map((c) => ({
                    name: c,
                    value: counts[c],
                  }))}
                  dataKey="value"
                  nameKey="name"
                  innerRadius={42}
                  outerRadius={66}
                  paddingAngle={1}
                  isAnimationActive={false}
                  stroke="#0A0A0A"
                  strokeWidth={1}
                >
                  {ORDER.filter((c) => counts[c] > 0).map((c) => (
                    <Cell key={c} fill={FILL_HEX[c]} />
                  ))}
                </Pie>
                <Tooltip
                  cursor={{ fill: "transparent" }}
                  contentStyle={{
                    background: "#0A0A0A",
                    border: "1px solid #2A2A2A",
                    fontFamily: "monospace",
                    fontSize: 11,
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
            {/* Center label: dominant category's share. */}
            <DonutCenter counts={counts} total={total} />
          </div>

          {/* Legend — counts + percentages, sorted desc so the top
              row is the dominant cause. */}
          <ul className="space-y-1">
            {ORDER.filter((c) => counts[c] > 0)
              .sort((a, b) => counts[b] - counts[a])
              .map((c) => {
                const pct = (counts[c] / total) * 100;
                return (
                  <li
                    key={c}
                    className="flex items-center gap-2 font-mono text-meta"
                  >
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${FILL[c]}`}
                      aria-hidden
                    />
                    <span className="text-text-secondary">{c}</span>
                    <span className="ml-auto shrink-0 tabular-nums text-text-tertiary">
                      {counts[c]} · {pct.toFixed(0)}%
                    </span>
                  </li>
                );
              })}
          </ul>
        </div>
      )}
      {counts && total === 0 && (
        <p className="font-mono text-meta text-text-tertiary">
          No RCAs found across the sample.
        </p>
      )}
    </section>
  );
}

// DonutCenter — overlays the dominant category's share as a big
// numeral inside the donut's hole. Counts is non-empty when this
// renders.
function DonutCenter({
  counts,
  total,
}: {
  counts: Record<RootCauseCategory, number>;
  total: number;
}) {
  const top = (Object.entries(counts) as Array<[RootCauseCategory, number]>)
    .filter(([, n]) => n > 0)
    .sort((a, b) => b[1] - a[1])[0];
  if (!top) return null;
  const pct = ((top[1] / total) * 100).toFixed(0);
  return (
    <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
      <span className="font-mono text-stat font-medium tabular-nums text-text-primary">
        {pct}%
      </span>
      <span className="mt-0.5 truncate max-w-[120px] text-center font-mono text-meta uppercase tracking-[0.05em] text-text-tertiary">
        {top[0].replace(/_/g, " ")}
      </span>
    </div>
  );
}
