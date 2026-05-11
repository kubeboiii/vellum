// Post-mortem author's home (PRD §6.3).
//
// Layout:
//   - RCA quality stats strip (% closed with RCA, avg fix length,
//     avg prevention length, RCAs by author)
//   - Queue of RESOLVED-waiting-for-RCA incidents, oldest first.
//     Each card → "Write RCA →" → /incidents/{id}/rca
//
// Data sources:
//   - listIncidents() — active list, filter to status === "RESOLVED"
//   - listClosedIncidents() — for quality stats, fetched once
//   - lazy: getIncident per closed item to pull RCA stats (capped at 50)
//
// This page never blocks on the lazy fetch — quality stats show
// "computing…" until they land; the RESOLVED queue renders on
// first poll.

"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { Nav } from "@/components/Nav";
import { SeverityBadge } from "@/components/SeverityBadge";
import { getIncident, listClosedIncidents, listIncidents } from "@/lib/api";
import type { RCA, WorkItem } from "@/lib/types";

const POLL_INTERVAL_MS = 5000;
const RCA_SAMPLE_CAP = 50;

interface QualityStats {
  closedTotal: number;
  withRCA: number;
  avgFixLen: number;
  avgPreventionLen: number;
  authors: Array<{ name: string; count: number }>;
}

export default function PostmortemPage() {
  const [resolved, setResolved] = useState<WorkItem[]>([]);
  const [closed, setClosed] = useState<WorkItem[] | null>(null);
  const [stats, setStats] = useState<QualityStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Poll resolved-list every 5s; closed list once on mount.
  useEffect(() => {
    let cancelled = false;
    async function poll() {
      try {
        const d = await listIncidents();
        if (!cancelled) {
          setResolved(d.items.filter((i) => i.status === "RESOLVED"));
          setError(null);
        }
      } catch (e) {
        if (!cancelled) setError((e as Error).message);
      }
    }
    poll();
    const id = setInterval(poll, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    listClosedIncidents(100)
      .then((d) => {
        if (!cancelled) setClosed(d.items);
      })
      .catch((e) => {
        if (!cancelled) setError((e as Error).message);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Lazy quality stats — compute when `closed` lands.
  useEffect(() => {
    if (!closed) return;
    let cancelled = false;
    (async () => {
      const sample = closed.slice(0, RCA_SAMPLE_CAP);
      const rcas: RCA[] = [];
      const results = await Promise.all(
        sample.map((wi) =>
          getIncident(wi.id)
            .then((d) => d.rca)
            .catch(() => undefined),
        ),
      );
      for (const r of results) if (r) rcas.push(r);
      if (cancelled) return;
      const authorCounts = new Map<string, number>();
      let fixSum = 0;
      let preventionSum = 0;
      for (const r of rcas) {
        fixSum += r.fix_applied.length;
        preventionSum += r.prevention_steps.length;
        authorCounts.set(
          r.submitted_by,
          (authorCounts.get(r.submitted_by) ?? 0) + 1,
        );
      }
      const authors = Array.from(authorCounts.entries())
        .map(([name, count]) => ({ name, count }))
        .sort((a, b) => b.count - a.count)
        .slice(0, 5);
      setStats({
        closedTotal: closed.length,
        withRCA: rcas.length,
        avgFixLen: rcas.length ? Math.round(fixSum / rcas.length) : 0,
        avgPreventionLen: rcas.length
          ? Math.round(preventionSum / rcas.length)
          : 0,
        authors,
      });
    })();
    return () => {
      cancelled = true;
    };
  }, [closed]);

  // Sort RESOLVED by last_signal_ts asc (oldest first — those need
  // the RCA most urgently).
  const queue = useMemo(
    () =>
      [...resolved].sort(
        (a, b) =>
          new Date(a.last_signal_ts).getTime() -
          new Date(b.last_signal_ts).getTime(),
      ),
    [resolved],
  );

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Post-mortem" />
      <HealthStrip />
      <main className="mx-auto max-w-[1200px] space-y-4 px-6 py-4">
        <header>
          <h1 className="relative pb-2 font-sans text-page font-semibold text-text-primary">
            Post-mortem queue
            <span
              className="absolute bottom-0 left-0 h-px w-6 bg-accent"
              aria-hidden
            />
          </h1>
          <p className="mt-2 font-mono text-meta text-text-tertiary">
            RESOLVED incidents waiting for an RCA. Closing without an RCA
            is impossible — the state machine rejects it (FR-4.5).
          </p>
        </header>

        {error && (
          <div className="rounded-sm border border-sev-p0-border bg-sev-p0-bg/40 px-3 py-2 font-mono text-meta text-red-300">
            {error}
          </div>
        )}

        <QualityStatsStrip stats={stats} closedTotal={closed?.length ?? null} />

        <section className="rounded-md border border-border-subtle bg-bg-surface">
          <header className="flex items-center justify-between border-b border-border-subtle bg-bg-elevated px-4 py-2">
            <h2 className="font-sans text-card font-semibold text-text-primary">
              Awaiting RCA
            </h2>
            <span className="font-mono text-meta text-text-tertiary tabular-nums">
              {queue.length} {queue.length === 1 ? "incident" : "incidents"}
            </span>
          </header>
          {queue.length === 0 ? (
            <div className="px-4 py-12 text-center font-mono text-meta text-text-tertiary">
              Inbox zero. Nothing waiting for an RCA right now.
            </div>
          ) : (
            <ul className="divide-y divide-border-subtle">
              {queue.map((wi) => (
                <ResolvedCard key={wi.id} wi={wi} />
              ))}
            </ul>
          )}
        </section>
      </main>
    </div>
  );
}

function QualityStatsStrip({
  stats,
  closedTotal,
}: {
  stats: QualityStats | null;
  closedTotal: number | null;
}) {
  const rcaPct =
    stats && stats.closedTotal > 0
      ? (stats.withRCA / stats.closedTotal) * 100
      : null;
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <StatTile
        label="RCA coverage"
        value={
          rcaPct === null
            ? "…"
            : `${rcaPct.toFixed(0)}%`
        }
        sub={
          stats
            ? `${stats.withRCA}/${stats.closedTotal} sampled`
            : closedTotal !== null
              ? "computing…"
              : "loading…"
        }
        tone={rcaPct !== null && rcaPct >= 95 ? "good" : "neutral"}
      />
      <StatTile
        label="Avg fix length"
        value={stats ? `${stats.avgFixLen}` : "…"}
        sub="chars"
      />
      <StatTile
        label="Avg prevention length"
        value={stats ? `${stats.avgPreventionLen}` : "…"}
        sub="chars"
      />
      <StatTile
        label="Top author"
        value={stats?.authors[0]?.name ?? "…"}
        sub={stats?.authors[0] ? `${stats.authors[0].count} RCAs` : ""}
      />
    </div>
  );
}

function StatTile({
  label,
  value,
  sub,
  tone = "neutral",
}: {
  label: string;
  value: string;
  sub?: string;
  tone?: "neutral" | "good";
}) {
  const valueClass =
    tone === "good" ? "text-accent" : "text-text-primary";
  return (
    <div className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <div className="flex items-center gap-1.5 font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        <span
          className="h-1 w-1 animate-pulse-live rounded-full bg-accent"
          aria-hidden
        />
        {label}
      </div>
      <div
        className={`mt-1 truncate font-mono text-stat font-medium tabular-nums ${valueClass}`}
      >
        {value}
      </div>
      {sub && (
        <div className="mt-1 font-mono text-meta text-text-tertiary">{sub}</div>
      )}
    </div>
  );
}

function ResolvedCard({ wi }: { wi: WorkItem }) {
  const resolvedAt = wi.last_signal_ts;
  const ageMs = Date.now() - new Date(resolvedAt).getTime();
  return (
    <li className="flex items-center gap-4 px-4 py-3 transition-colors duration-fast hover:bg-bg-hover">
      <SeverityBadge severity={wi.severity} />
      <div className="min-w-0 flex-1">
        <div className="truncate font-mono text-card text-text-primary">
          {wi.component_id}
          <span className="ml-2 font-mono text-meta text-text-tertiary">
            {wi.id.slice(0, 8)}
          </span>
        </div>
        <div className="mt-0.5 font-mono text-meta text-text-tertiary">
          {wi.component_type} · {wi.signal_count.toLocaleString()} signals · resolved {fmtAge(ageMs)} ago
        </div>
      </div>
      <Link
        href={`/incidents/${wi.id}/rca`}
        className="inline-flex shrink-0 items-center gap-1.5 rounded-sm bg-accent px-3 py-1.5 font-sans text-meta font-medium text-accent-text transition-[background-color,box-shadow] duration-fast ease-out hover:bg-accent-bright hover:shadow-[0_0_18px_-6px_rgba(190,242,100,0.55)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-base"
      >
        Write RCA ›
      </Link>
    </li>
  );
}

function fmtAge(ms: number): string {
  if (ms < 0) return "now";
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h`;
  return `${Math.floor(h / 24)}d`;
}
