// MTTRTrend — line chart of MTTR per closed incident, plotted in
// chronological order (closed_at asc). Tells the team whether
// resolution times are getting better or worse over time.

"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import type { WorkItem } from "@/lib/types";

interface Props {
  items: WorkItem[];
}

export function MTTRTrend({ items }: Props) {
  const points = items
    .filter(
      (i): i is WorkItem & { closed_at: string; mttr_seconds: number } =>
        typeof i.mttr_seconds === "number" && !!i.closed_at,
    )
    .sort(
      (a, b) => new Date(a.closed_at).getTime() - new Date(b.closed_at).getTime(),
    )
    .map((i) => ({
      // x is "minutes since the oldest in the window" — keeps the
      // axis legible when incidents span days.
      t: new Date(i.closed_at).getTime(),
      mttr: i.mttr_seconds,
      label: i.component_id,
    }));

  if (points.length < 2) {
    return (
      <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          MTTR trend
        </h3>
        <p className="mt-2 font-mono text-meta text-text-tertiary">
          Need ≥2 closed incidents to plot a trend. Currently{" "}
          {points.length}.
        </p>
      </section>
    );
  }

  // Use the raw timestamp as a numeric x so points are spaced by
  // real time (not insertion order). The previous design used
  // "minutes since the oldest in the window" which was opaque to
  // a human reader; we now format ticks as real dates/times.
  const first = points[0].t;
  const last = points[points.length - 1].t;
  const data = points.map((p) => ({
    t: p.t,
    mttr: p.mttr,
    label: p.label,
  }));
  const spanMs = last - first;
  // Pick a tick formatter that fits the span: short bursts show
  // hh:mm, multi-day windows show MMM d.
  const formatTick = (ms: number) => {
    const d = new Date(ms);
    if (spanMs < 24 * 60 * 60_000) {
      return d.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        hour12: false,
      });
    }
    return d.toLocaleDateString([], { month: "short", day: "numeric" });
  };
  const spanLabel = fmtSpan(spanMs);

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-2 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          MTTR trend
        </h3>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {data.length} closed · {spanLabel} span
        </span>
      </header>
      <div className="h-32">
        <ResponsiveContainer width="100%" height="100%">
          <LineChart data={data} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
            <CartesianGrid stroke="#2A2A2A" strokeDasharray="2 3" />
            <XAxis
              dataKey="t"
              type="number"
              domain={["dataMin", "dataMax"]}
              scale="time"
              stroke="#71717A"
              tick={{ fontSize: 10, fontFamily: "monospace", fill: "#71717A" }}
              tickLine={false}
              tickFormatter={formatTick}
              minTickGap={40}
            />
            <YAxis
              stroke="#71717A"
              tick={{ fontSize: 10, fontFamily: "monospace", fill: "#71717A" }}
              tickLine={false}
              width={40}
              tickFormatter={(v: number) => fmtMTTRShort(v)}
            />
            <Tooltip
              cursor={{ stroke: "#3F3F46" }}
              contentStyle={{
                background: "#0A0A0A",
                border: "1px solid #2A2A2A",
                fontFamily: "monospace",
                fontSize: 11,
              }}
              labelFormatter={(ms: unknown) =>
                new Date(Number(ms)).toLocaleString([], {
                  month: "short",
                  day: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })
              }
              formatter={(v: unknown) => [fmtMTTRShort(Number(v)), "MTTR"]}
            />
            <Line
              type="monotone"
              dataKey="mttr"
              stroke="#BEF264"
              strokeWidth={1.5}
              dot={{ r: 2, fill: "#BEF264" }}
              isAnimationActive={false}
            />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}

function fmtSpan(ms: number): string {
  if (ms < 60_000) return `${Math.round(ms / 1000)}s`;
  if (ms < 3_600_000) return `${Math.round(ms / 60_000)}m`;
  if (ms < 86_400_000) return `${Math.round(ms / 3_600_000)}h`;
  return `${Math.round(ms / 86_400_000)}d`;
}

function fmtMTTRShort(s: number): string {
  if (s < 60) return `${Math.round(s)}s`;
  if (s < 3600) return `${Math.round(s / 60)}m`;
  return `${(s / 3600).toFixed(1)}h`;
}
