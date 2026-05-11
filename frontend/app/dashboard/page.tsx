"use client";

import { IconSearch } from "@tabler/icons-react";
import { AnimatePresence } from "framer-motion";
import { useEffect, useMemo, useRef, useState } from "react";

import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { IncidentRateStrip } from "@/components/dashboard/IncidentRateStrip";
import { NoisyComponents } from "@/components/dashboard/NoisyComponents";
import { PersonaSwitcher } from "@/components/dashboard/PersonaSwitcher";
import { SeverityStackedBar } from "@/components/dashboard/SeverityStackedBar";
import { useP0Beep } from "@/components/dashboard/useP0Beep";
import { IncidentRow } from "@/components/IncidentRow";
import { Nav } from "@/components/Nav";
import { SignalRateChart, type RateBucket } from "@/components/SignalRateChart";
import { StatCard } from "@/components/StatCard";
import { listIncidents } from "@/lib/api";
import type { Persona } from "@/lib/persona";
import type { Severity, Status, WorkItem } from "@/lib/types";

const POLL_INTERVAL_MS = 2000;
const HISTORY_BUCKETS = 30;

type Filter = "ALL" | Severity;

interface Series {
  active: number[];
  p0: number[];
  ingest: number[];
  mttr: number[];
}

const emptySeries: Series = {
  active: Array(HISTORY_BUCKETS).fill(0),
  p0: Array(HISTORY_BUCKETS).fill(0),
  ingest: Array(HISTORY_BUCKETS).fill(0),
  mttr: Array(HISTORY_BUCKETS).fill(0),
};

export default function LiveFeed() {
  const [items, setItems] = useState<WorkItem[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<Filter>("ALL");
  const [search, setSearch] = useState("");
  const [muted, setMuted] = useState(true);

  const [persona, setPersona] = useState<Persona>("sre");

  useP0Beep(items, muted);

  const [series, setSeries] = useState<Series>(emptySeries);
  const [buckets, setBuckets] = useState<RateBucket[]>([]);
  const lastSignalsRef = useRef<number | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function poll() {
      try {
        const data = await listIncidents();
        if (cancelled) return;

        setItems(data.items);
        setError(null);

        const now = new Date();
        const activeCount = data.items.length;
        const p0Count = data.items.filter((i) => i.severity === "P0").length;
        const totalSignals = data.items.reduce((a, x) => a + x.signal_count, 0);
        const ingestRate =
          lastSignalsRef.current == null
            ? 0
            : Math.max(0, (totalSignals - lastSignalsRef.current) / (POLL_INTERVAL_MS / 1000));
        lastSignalsRef.current = totalSignals;

        setSeries((s) => ({
          active: shiftPush(s.active, activeCount),
          p0: shiftPush(s.p0, p0Count),
          ingest: shiftPush(s.ingest, ingestRate),
          mttr: shiftPush(s.mttr, avgMTTR(data.items)),
        }));

        const bucket: RateBucket = {
          t: now.toISOString(),
          p0: data.items
            .filter((i) => i.severity === "P0")
            .reduce((a, x) => a + x.signal_count, 0),
          p1: data.items
            .filter((i) => i.severity === "P1")
            .reduce((a, x) => a + x.signal_count, 0),
          p2: data.items
            .filter((i) => i.severity === "P2")
            .reduce((a, x) => a + x.signal_count, 0),
          p3: data.items
            .filter((i) => i.severity === "P3")
            .reduce((a, x) => a + x.signal_count, 0),
          other: 0,
        };
        setBuckets((b) => {
          const next = [...b, bucket];
          if (next.length > HISTORY_BUCKETS) next.shift();
          return next;
        });
      } catch (e) {
        if (!cancelled) setError((e as Error).message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    poll();
    const id = setInterval(poll, POLL_INTERVAL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    const personaPreFilter: (wi: WorkItem) => boolean =
      persona === "postmortem"
        ? (wi) => wi.status === "RESOLVED"
        : () => true;
    return items.filter((i) => {
      if (!personaPreFilter(i)) return false;
      if (filter !== "ALL" && i.severity !== filter) return false;

      if (q && !i.component_id.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [items, filter, search, persona]);

  const currentRate = series.ingest[series.ingest.length - 1] ?? 0;

  const stateCounts: Record<Status, number> = items.reduce(
    (acc, wi) => {
      acc[wi.status] = (acc[wi.status] ?? 0) + 1;
      return acc;
    },
    { OPEN: 0, INVESTIGATING: 0, RESOLVED: 0, CLOSED: 0 } as Record<Status, number>,
  );

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Live Feed" muted={muted} onToggleMute={() => setMuted((m) => !m)} />
      <HealthStrip />

      <main className="mx-auto max-w-[1400px] px-6 py-4 space-y-4">
        {}
        <PersonaSwitcher value={persona} onChange={setPersona} />

        {}
        {persona === "commander" && (
          <StateCountsStrip counts={stateCounts} />
        )}

        {}
        <SignalRateChart buckets={buckets} currentRatePerSec={Math.round(currentRate)} />

        {}
        <SeverityStackedBar items={items} />

        {}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Active Incidents"
            value={items.length}
            delta={deltaLabel(series.active)}
            deltaTone={deltaTone(series.active, "up-is-bad")}
            sparkline={series.active}
            sparkColor="#BEF264"
          />
          <StatCard
            label="P0"
            value={items.filter((i) => i.severity === "P0").length}
            delta={deltaLabel(series.p0)}
            deltaTone={deltaTone(series.p0, "up-is-bad")}
            sparkline={series.p0}
            sparkColor="#EF4444"
          />
          <StatCard
            label="Avg MTTR (closed)"
            value={fmtMTTR(series.mttr[series.mttr.length - 1] ?? 0)}
            sparkline={series.mttr}
            sparkColor="#71717A"
          />
          <StatCard
            label="Ingest Rate"
            value={`${Math.round(currentRate).toLocaleString()}/s`}
            sparkline={series.ingest}
            sparkColor="#BEF264"
          />
        </div>

        {}
        <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
          <NoisyComponents items={items} />
          <IncidentRateStrip items={items} />
        </div>

        {}
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="mr-2 font-sans text-label uppercase tracking-[0.05em] text-text-secondary">
            Filters
          </span>
          {(["ALL", "P0", "P1", "P2", "P3"] as const).map((f) => (
            <button
              key={f}
              onClick={() => setFilter(f)}
              className={`rounded-sm border px-2 py-0.5 font-mono text-meta font-medium uppercase tracking-[0.04em] transition-colors duration-fast ${
                filter === f
                  ? "border-accent bg-accent-bg text-accent"
                  : "border-border-subtle bg-transparent text-text-secondary hover:bg-bg-elevated"
              }`}
            >
              {f}
            </button>
          ))}
          {}
          <div className="ml-auto flex items-center gap-2 rounded-sm border border-border-subtle bg-bg-input px-2 py-0.5 focus-within:border-border-focus">
            <IconSearch size={12} className="text-text-tertiary" aria-hidden />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="search component_id…"
              className="w-56 bg-transparent font-mono text-meta text-text-primary placeholder:text-text-tertiary focus-visible:outline-none"
            />
            {search && (
              <button
                onClick={() => setSearch("")}
                className="font-mono text-meta text-text-tertiary transition-colors duration-fast hover:text-text-primary"
                aria-label="clear search"
              >
                ✕
              </button>
            )}
          </div>
        </div>

        {}
        <div className="flex items-center justify-between pt-2">
          <h2 className="relative font-sans text-section font-semibold text-text-primary">
            Active Incidents
            <span className="absolute -bottom-1 left-0 h-px w-6 bg-accent" aria-hidden />
          </h2>
          <span className="font-mono text-meta text-text-tertiary tabular-nums">
            {filtered.length === items.length ? (
              <>
                {filtered.length}{" "}
                {filtered.length === 1 ? "incident" : "incidents"}
              </>
            ) : (
              <>
                <span className="text-text-primary">{filtered.length}</span>{" "}
                of {items.length} shown
              </>
            )}
          </span>
        </div>

        {}
        {filtered.length !== items.length && (
          <div className="flex flex-wrap items-center gap-2 rounded-sm border border-sev-p1-border bg-sev-p1-bg/40 px-3 py-2 font-mono text-meta text-sev-p1">
            <span>
              Hiding {items.length - filtered.length} of {items.length}{" "}
              incidents due to active filters:
            </span>
            {persona !== "sre" && (
              <span className="rounded-sm border border-sev-p1-border bg-bg-base px-1.5 py-0.5">
                persona = {persona}
              </span>
            )}
            {filter !== "ALL" && (
              <span className="rounded-sm border border-sev-p1-border bg-bg-base px-1.5 py-0.5">
                severity = {filter}
              </span>
            )}
            {search && (
              <span className="rounded-sm border border-sev-p1-border bg-bg-base px-1.5 py-0.5">
                search = &quot;{search}&quot;
              </span>
            )}
            <button
              type="button"
              onClick={() => {
                setPersona("sre");
                setFilter("ALL");
                setSearch("");
              }}
              className="ml-auto rounded-sm border border-sev-p1-border bg-bg-base px-2 py-0.5 text-sev-p1 transition-colors duration-fast hover:bg-sev-p1-bg"
            >
              Clear filters
            </button>
          </div>
        )}

        {error && (
          <div className="rounded-sm border border-sev-p0-border bg-sev-p0-bg/40 px-3 py-2 font-mono text-meta text-red-300">
            {error}
          </div>
        )}

        {}
        <div className="overflow-hidden rounded-md border border-border-subtle bg-bg-surface">
          {}
          <div className="grid h-7 grid-cols-[12px_1fr_56px_140px_120px_100px_90px_20px] items-center gap-3 border-b border-border-subtle bg-bg-elevated px-4 font-sans text-label uppercase tracking-[0.05em] text-text-tertiary">
            <span aria-hidden />
            <span>Component</span>
            <span>Sev</span>
            <span>State</span>
            <span className="text-right">Signals</span>
            <span>Time</span>
            <span>Age</span>
            <span aria-hidden />
          </div>

          {loading && items.length === 0 ? (
            <div className="px-4 py-8 font-mono text-meta text-text-tertiary">
              Loading incidents…
            </div>
          ) : filtered.length === 0 ? (
            <div className="px-4 py-8 font-mono text-meta text-text-tertiary">
              No active incidents
              {filter !== "ALL" ? ` at severity ${filter}` : ""}
              {search ? ` matching “${search}”` : ""}. Send a signal to{" "}
              <span className="text-text-secondary">POST /v1/signals</span>{" "}
              to create one.
            </div>
          ) : (

            <AnimatePresence initial={false}>
              {filtered.map((wi) => <IncidentRow key={wi.id} wi={wi} />)}
            </AnimatePresence>
          )}
        </div>

        {}
        <MetricsTerminal
          currentRate={currentRate}
          activeCount={items.length}
          p0Count={items.filter((i) => i.severity === "P0").length}
        />
      </main>
    </div>
  );
}

function StateCountsStrip({
  counts,
}: {
  counts: Record<Status, number>;
}) {
  const order: Array<{ k: Status; tone: string; bg: string }> = [
    { k: "OPEN", tone: "text-sev-p0", bg: "bg-sev-p0" },
    { k: "INVESTIGATING", tone: "text-sev-p1", bg: "bg-sev-p1" },
    { k: "RESOLVED", tone: "text-accent", bg: "bg-accent" },
    { k: "CLOSED", tone: "text-text-tertiary", bg: "bg-text-tertiary" },
  ];
  return (
    <section className="grid grid-cols-2 gap-3 sm:grid-cols-4">
      {order.map(({ k, tone, bg }) => (
        <div
          key={k}
          className="rounded-md border border-border-subtle bg-bg-surface p-3"
        >
          <div className="flex items-center gap-1.5 font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            <span
              className={`h-1.5 w-1.5 rounded-full ${bg} ${k === "OPEN" && counts[k] > 0 ? "animate-pulse-live" : ""}`}
              aria-hidden
            />
            {k.toLowerCase()}
          </div>
          <div className={`mt-1 font-mono text-stat font-medium tabular-nums ${tone}`}>
            {counts[k]}
          </div>
        </div>
      ))}
    </section>
  );
}

function MetricsTerminal({
  currentRate,
  activeCount,
  p0Count,
}: {
  currentRate: number;
  activeCount: number;
  p0Count: number;
}) {
  return (
    <div className="overflow-hidden rounded-md border border-border-subtle bg-bg-surface">
      {}
      <div className="flex h-7 items-center gap-2 border-b border-border-subtle bg-bg-elevated px-3">
        <span className="flex gap-1.5" aria-hidden>
          <span className="h-2 w-2 rounded-full bg-[#3F3F46]" />
          <span className="h-2 w-2 rounded-full bg-[#3F3F46]" />
          <span className="h-2 w-2 rounded-full bg-[#3F3F46]" />
        </span>
        <span className="font-mono text-meta text-text-tertiary">
          /var/log/vellum/metrics.log
        </span>
        <span className="ml-auto inline-flex items-center gap-1.5 font-mono text-meta uppercase tracking-[0.05em] text-accent">
          <span className="h-1 w-1 animate-pulse-live rounded-full bg-accent" aria-hidden />
          LIVE · 2s
        </span>
      </div>
      {}
      <div className="px-3 py-2 font-mono text-meta text-text-tertiary">
        <span className="text-text-tertiary">[metrics]</span>{" "}
        accepted=<span className="text-accent tabular-nums">{Math.round(currentRate).toLocaleString()}/s</span>{" "}
        · queue=<span className="text-text-secondary tabular-nums">—/50000</span>{" "}
        · active=<span className="text-text-primary tabular-nums">{activeCount}</span>{" "}
        · p0=<span className={`tabular-nums ${p0Count > 0 ? "text-sev-p0" : "text-text-secondary"}`}>{p0Count}</span>
      </div>
    </div>
  );
}

function shiftPush(arr: number[], n: number): number[] {
  const next = arr.slice(1);
  next.push(n);
  return next;
}

function deltaLabel(series: number[]): string | undefined {
  if (series.length < 2) return undefined;
  const now = series[series.length - 1];
  const prev = series[series.length - 2];
  const d = now - prev;
  if (d === 0) return "no change";
  return `${d > 0 ? "+" : ""}${d.toFixed(0)} from prev`;
}

function deltaTone(
  series: number[],
  semantics: "up-is-good" | "up-is-bad",
): "good" | "bad" | "neutral" {
  if (series.length < 2) return "neutral";
  const d = series[series.length - 1] - series[series.length - 2];
  if (d === 0) return "neutral";
  if (semantics === "up-is-bad") return d > 0 ? "bad" : "good";
  return d > 0 ? "good" : "bad";
}

function avgMTTR(items: WorkItem[]): number {
  const closed = items.filter((i) => i.mttr_seconds != null);
  if (closed.length === 0) return 0;
  return closed.reduce((a, x) => a + (x.mttr_seconds ?? 0), 0) / closed.length;
}

function fmtMTTR(seconds: number): string {
  if (seconds === 0) return "—";
  if (seconds < 60) return `${Math.round(seconds)}s`;
  const m = Math.floor(seconds / 60);
  if (m < 60) return `${m}m`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}
