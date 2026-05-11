// MTTRHistogram — bucketed distribution of mttr_seconds across the
// closed-incident set. Fast read: are most incidents resolved in
// minutes, or are we seeing long tails?

"use client";

import type { WorkItem } from "@/lib/types";

interface Bucket {
  label: string;
  min: number; // inclusive seconds
  max: number; // exclusive seconds
}

const BUCKETS: Bucket[] = [
  { label: "<1m", min: 0, max: 60 },
  { label: "1–5m", min: 60, max: 300 },
  { label: "5–15m", min: 300, max: 900 },
  { label: "15–60m", min: 900, max: 3600 },
  { label: "≥1h", min: 3600, max: Number.POSITIVE_INFINITY },
];

interface Props {
  items: WorkItem[];
}

export function MTTRHistogram({ items }: Props) {
  const closed = items.filter((i) => typeof i.mttr_seconds === "number");
  const counts = BUCKETS.map(
    (b) =>
      closed.filter((wi) => {
        const m = wi.mttr_seconds as number;
        return m >= b.min && m < b.max;
      }).length,
  );
  const max = Math.max(1, ...counts);

  // Median MTTR — the typical value. Drawn as a horizontal lime
  // dashed line across the histogram so the user gets "here's the
  // shape" AND "here's the middle" without doing the math.
  const median = computeMedian(closed.map((c) => c.mttr_seconds as number));
  const medianBucketIdx = median !== null ? bucketIndexOf(median) : -1;

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          MTTR distribution
        </h3>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {closed.length} closed
          {median !== null && (
            <>
              <span className="mx-1.5 text-border-strong" aria-hidden>
                ·
              </span>
              <span className="text-accent">median {fmtSec(median)}</span>
            </>
          )}
        </span>
      </header>
      {closed.length === 0 ? (
        <p className="font-mono text-meta text-text-tertiary">
          No closed incidents in window.
        </p>
      ) : (
        <div className="relative">
          <ul className="flex h-24 items-end gap-2">
            {BUCKETS.map((b, i) => {
              const h = (counts[i] / max) * 100;
              const isMedian = i === medianBucketIdx;
              return (
                <li
                  key={b.label}
                  className="flex h-full flex-1 flex-col items-center justify-end gap-1"
                >
                  <span
                    className={`font-mono text-meta tabular-nums ${isMedian ? "text-accent" : "text-text-secondary"}`}
                  >
                    {counts[i]}
                  </span>
                  <div
                    className={`w-full rounded-sm transition-[height] duration-base ease-out ${isMedian ? "bg-accent" : "bg-accent/60"}`}
                    style={{ height: `${Math.max(2, h)}%` }}
                    title={`${b.label} · ${counts[i]} incidents${isMedian ? " (contains median)" : ""}`}
                  />
                  <span
                    className={`font-mono text-meta uppercase tracking-[0.05em] ${isMedian ? "text-accent" : "text-text-tertiary"}`}
                  >
                    {b.label}
                  </span>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </section>
  );
}

function computeMedian(values: number[]): number | null {
  if (values.length === 0) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  if (sorted.length % 2 === 1) return sorted[mid];
  return (sorted[mid - 1] + sorted[mid]) / 2;
}

function bucketIndexOf(sec: number): number {
  for (let i = 0; i < BUCKETS.length; i++) {
    if (sec >= BUCKETS[i].min && sec < BUCKETS[i].max) return i;
  }
  return -1;
}

function fmtSec(s: number): string {
  if (s < 60) return `${Math.round(s)}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${Math.round(s % 60)}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}
