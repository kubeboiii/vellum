// THEME.md §6.8 — Hero signal-rate chart.
//
// "Compact but visually significant — when the failure simulator
// runs, this is what visibly spikes in the demo video."
//
// Spec recap:
//   - 120px tall, full width
//   - Stacked AreaChart by severity (P0 red on top, then P1/P2/P3,
//     resolved/closed in gray at the bottom)
//   - Fills at low alpha (0.20), top stroke at full color, 1px
//   - 15-minute window, 30s buckets (30 points)
//   - X-axis: only "15m ago" and "now" labels
//   - Y-axis: hidden; current rate in top-right with live lime dot
//   - Background: bg-surface, 1px border, radius-md, padding 12/16
//
// The chart is fed a window of bucketed data points by the parent.
// Phase 5 v1: the parent constructs a 30-point window by polling
// the metrics ticker line and bucketing client-side. (A proper
// /v1/metrics/rate endpoint is Phase 6+.)

"use client";

import { Area, AreaChart, ResponsiveContainer, Tooltip, XAxis } from "recharts";

export interface RateBucket {
  // Bucket-end timestamp; we use it as the X axis category.
  t: string;
  p0: number;
  p1: number;
  p2: number;
  p3: number;
  other: number;
}

interface SignalRateChartProps {
  buckets: RateBucket[];
  currentRatePerSec: number;
}

// Colors mirror THEME.md §2.3 severity hues.
const SEVERITY_FILLS: Array<{ key: keyof RateBucket; stroke: string; fill: string }> = [
  { key: "other", stroke: "#71717A", fill: "rgba(113,113,122,0.20)" },
  { key: "p3", stroke: "#3B82F6", fill: "rgba(59,130,246,0.20)" },
  { key: "p2", stroke: "#F59E0B", fill: "rgba(245,158,11,0.20)" },
  { key: "p1", stroke: "#F97316", fill: "rgba(249,115,22,0.20)" },
  { key: "p0", stroke: "#EF4444", fill: "rgba(239,68,68,0.20)" },
];

export function SignalRateChart({ buckets, currentRatePerSec }: SignalRateChartProps) {
  // Empty state shows the frame so the dashboard layout doesn't
  // jump when the first data arrives.
  const data = buckets.length > 0 ? buckets : Array.from({ length: 30 }, (_, i) => ({
    t: String(i), p0: 0, p1: 0, p2: 0, p3: 0, other: 0,
  }));

  return (
    <div className="rounded-md border border-border-subtle bg-bg-surface p-3">
      <div className="mb-2 flex items-center justify-between font-mono text-meta uppercase tracking-[0.04em] text-text-secondary">
        <span>signal rate · last 15 min</span>
        <span className="flex items-center gap-2">
          <span className="text-text-primary tabular-nums">
            {currentRatePerSec.toLocaleString()}/s
          </span>
          <span className="inline-block h-1.5 w-1.5 rounded-full bg-accent animate-pulse-live" aria-hidden />
          <span className="text-text-tertiary">live</span>
        </span>
      </div>
      <div className="h-[96px] w-full">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 4, right: 0, left: 0, bottom: 8 }}>
            {SEVERITY_FILLS.map(({ key, stroke, fill }) => (
              <Area
                key={key as string}
                type="monotone"
                dataKey={key as string}
                stackId="1"
                stroke={stroke}
                strokeWidth={1}
                fill={fill}
                isAnimationActive={false}
              />
            ))}
            <XAxis
              dataKey="t"
              ticks={data.length > 1 ? [data[0].t, data[data.length - 1].t] : []}
              tickFormatter={(_, i) => (i === 0 ? "15m ago" : "now")}
              axisLine={false}
              tickLine={false}
              interval="preserveStartEnd"
              tick={{ fill: "#71717A", fontSize: 11, fontFamily: "var(--font-mono)" }}
              height={16}
            />
            <Tooltip
              cursor={{ stroke: "#404040", strokeWidth: 1 }}
              contentStyle={{
                background: "#141414",
                border: "1px solid #2A2A2A",
                borderRadius: 4,
                fontSize: 11,
                fontFamily: "var(--font-mono)",
                color: "#FAFAFA",
              }}
              labelStyle={{ color: "#A1A1AA" }}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
