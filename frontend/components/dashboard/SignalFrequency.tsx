"use client";

import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import type { Signal } from "@/lib/types";

interface Props {
  signals: Signal[];
}

const BINS = 12;

export function SignalFrequency({ signals }: Props) {
  if (signals.length < 2) {
    return (
      <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Signal frequency
        </h3>
        <p className="mt-2 font-mono text-meta text-text-tertiary">
          Need ≥2 signals to plot a frequency. This incident has{" "}
          {signals.length}.
        </p>
      </section>
    );
  }

  const times = signals.map((s) => new Date(s.timestamp).getTime());
  const tmin = Math.min(...times);
  const tmax = Math.max(...times);

  const span = Math.max(1, tmax - tmin);
  const buckets = Array(BINS).fill(0);
  for (const t of times) {
    const idx = Math.min(BINS - 1, Math.floor(((t - tmin) / span) * BINS));
    buckets[idx] += 1;
  }

  const binMs = span / BINS;
  const data = buckets.map((y, i) => ({
    i,
    y,
    label: fmtOffset(i * binMs),
  }));
  const spanLabel = fmtSpan(span);

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-2 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Signal frequency
        </h3>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {signals.length} signals · {spanLabel} span
        </span>
      </header>
      <div className="h-28">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 4, right: 4, left: -20, bottom: 0 }}>
            <XAxis
              dataKey="label"
              stroke="#71717A"
              tick={{ fontSize: 10, fontFamily: "monospace", fill: "#71717A" }}
              tickLine={false}
              interval={2}
            />
            <YAxis
              stroke="#71717A"
              tick={{ fontSize: 10, fontFamily: "monospace", fill: "#71717A" }}
              tickLine={false}
              width={32}
              allowDecimals={false}
            />
            <Tooltip
              cursor={{ fill: "rgba(255,255,255,0.04)" }}
              contentStyle={{
                background: "#0A0A0A",
                border: "1px solid #2A2A2A",
                fontFamily: "monospace",
                fontSize: 11,
              }}
              labelFormatter={(label: unknown) =>
                `bin ${String(label)} (${fmtSpan(binMs)} wide)`
              }
            />
            <Bar dataKey="y" fill="#BEF264" isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}

function fmtOffset(ms: number): string {
  if (ms < 1000) return `+${Math.round(ms)}ms`;
  const s = ms / 1000;
  if (s < 60) return `+${s.toFixed(s < 10 ? 1 : 0)}s`;
  const m = s / 60;
  return `+${m.toFixed(m < 10 ? 1 : 0)}m`;
}

function fmtSpan(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}
