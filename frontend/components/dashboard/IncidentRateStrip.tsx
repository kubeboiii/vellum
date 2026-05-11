"use client";

import { useEffect, useState } from "react";
import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

import type { Severity, WorkItem } from "@/lib/types";

interface Props {
  items: WorkItem[];
}

const WINDOW_MIN = 15;

const SEV_COLOR: Record<Severity, string> = {
  P0: "#EF4444",
  P1: "#F59E0B",
  P2: "#BEF264",
  P3: "#71717A",
};

interface Bucket {

  m: string;

  m_idx: number;
  P0: number;
  P1: number;
  P2: number;
  P3: number;
}

export function IncidentRateStrip({ items }: Props) {

  const [now, setNow] = useState<number | null>(null);
  useEffect(() => {
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), 30_000);
    return () => clearInterval(id);
  }, []);

  if (now === null) {
    return (
      <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
        <header className="mb-3 flex items-center justify-between">
          <h2 className="font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
            New incidents · last {WINDOW_MIN} min
          </h2>
        </header>
        {}
        <div className="h-24" aria-hidden />
      </section>
    );
  }

  const buckets: Bucket[] = [];
  for (let i = 0; i < WINDOW_MIN; i++) {
    const minutesAgo = WINDOW_MIN - 1 - i;
    const minStart = now - (minutesAgo + 1) * 60_000;
    const minEnd = now - minutesAgo * 60_000;
    const inMin = items.filter((wi) => {
      const ts = new Date(wi.first_signal_ts).getTime();
      return ts >= minStart && ts < minEnd;
    });
    const b: Bucket = {
      m_idx: -minutesAgo,
      m: minutesAgo === 0 ? "now" : `-${minutesAgo}m`,
      P0: 0,
      P1: 0,
      P2: 0,
      P3: 0,
    };
    for (const wi of inMin) b[wi.severity] += 1;
    buckets.push(b);
  }

  const totalInWindow = buckets.reduce(
    (a, b) => a + b.P0 + b.P1 + b.P2 + b.P3,
    0,
  );

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <h2 className="font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
          New incidents · last {WINDOW_MIN} min
        </h2>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {totalInWindow} total
        </span>
      </header>
      <div className="h-24">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={buckets}
            margin={{ top: 4, right: 4, left: -20, bottom: 0 }}
          >
            <XAxis
              dataKey="m"
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
              labelFormatter={(m) => `minute ${m}`}
            />
            {}
            <Bar dataKey="P0" stackId="sev" fill={SEV_COLOR.P0} isAnimationActive={false} />
            <Bar dataKey="P1" stackId="sev" fill={SEV_COLOR.P1} isAnimationActive={false} />
            <Bar dataKey="P2" stackId="sev" fill={SEV_COLOR.P2} isAnimationActive={false} />
            <Bar dataKey="P3" stackId="sev" fill={SEV_COLOR.P3} isAnimationActive={false} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}
