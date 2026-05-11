"use client";

import { IconChevronRight } from "@tabler/icons-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { CategoryBreakdown } from "@/components/dashboard/CategoryBreakdown";
import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { MTTRHistogram } from "@/components/dashboard/MTTRHistogram";
import { MTTRTrend } from "@/components/dashboard/MTTRTrend";
import { RepeatOffenders } from "@/components/dashboard/RepeatOffenders";
import { Nav } from "@/components/Nav";
import { SeverityBadge } from "@/components/SeverityBadge";
import { StatePill } from "@/components/StatePill";
import { listClosedIncidents } from "@/lib/api";
import type { Severity, WorkItem } from "@/lib/types";

const SEV_GLOW: Record<Severity, string> = {
  P0: "0 0 24px -10px rgba(239,68,68,0.55)",
  P1: "0 0 24px -10px rgba(245,158,11,0.45)",
  P2: "0 0 24px -10px rgba(190,242,100,0.40)",
  P3: "0 0 24px -10px rgba(113,113,122,0.40)",
};

function fmtMTTR(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) return "—";
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  if (m < 60) return `${m}m ${seconds % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

function fmtRelative(iso: string | undefined): string {
  if (!iso) return "—";
  const ms = Date.now() - new Date(iso).getTime();
  if (ms < 0) return "now";
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

export default function ClosedIncidentsPage() {
  const [items, setItems] = useState<WorkItem[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    (async () => {
      try {
        const data = await listClosedIncidents(100);
        setItems(data.items);
      } catch (e) {
        setError((e as Error).message);
      }
    })();
  }, []);

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="History" />
      <HealthStrip />
      <main className="mx-auto max-w-[1400px] space-y-4 px-6 py-4">
        <div className="flex items-end justify-between">
          <h1 className="relative pb-2 font-sans text-page font-semibold text-text-primary">
            Closed Incidents
            <span className="absolute bottom-0 left-0 h-px w-6 bg-accent" aria-hidden />
          </h1>
          <span className="font-mono text-meta text-text-tertiary">
            {items ? `${items.length} ${items.length === 1 ? "incident" : "incidents"}` : "—"}
          </span>
        </div>

        {}
        {items && items.length > 0 && (
          <>
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
              <MTTRHistogram items={items} />
              <RepeatOffenders items={items} />
            </div>
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
              <CategoryBreakdown items={items} />
              <MTTRTrend items={items} />
            </div>
          </>
        )}

        {error && (
          <div className="rounded-sm border border-sev-p0-border bg-sev-p0-bg/40 px-3 py-2 font-mono text-meta text-red-300">
            {error}
          </div>
        )}

        <div className="overflow-hidden rounded-md border border-border-subtle bg-bg-surface">
          {}
          <div className="grid h-7 grid-cols-[12px_1fr_56px_140px_120px_100px_90px_20px] items-center gap-3 border-b border-border-subtle bg-bg-elevated px-4 font-sans text-label uppercase tracking-[0.05em] text-text-tertiary">
            <span aria-hidden />
            <span>Component</span>
            <span>Sev</span>
            <span>State</span>
            <span className="text-right">Signals</span>
            <span>MTTR</span>
            <span>Closed</span>
            <span aria-hidden />
          </div>

          {items === null ? (
            <div className="flex items-center gap-2 px-4 py-8 font-mono text-meta text-text-tertiary">
              <span
                className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
                aria-hidden
              />
              Loading closed incidents…
            </div>
          ) : items.length === 0 ? (
            <div className="px-4 py-8 font-mono text-meta text-text-tertiary">
              No closed incidents yet. Once you submit an RCA, the incident
              moves here.
            </div>
          ) : (
            items.map((wi) => (
              <Link
                key={wi.id}
                href={`/incidents/${wi.id}`}
                className="group grid h-8 grid-cols-[12px_1fr_56px_140px_120px_100px_90px_20px] items-center gap-3 border-b border-border-subtle px-4 transition-[background-color,box-shadow] duration-fast ease-out hover:bg-bg-hover focus-visible:bg-bg-hover focus-visible:outline-none"

                onMouseEnter={(e) => {
                  e.currentTarget.style.boxShadow = SEV_GLOW[wi.severity];
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.boxShadow = "";
                }}
              >
                <span
                  className={`inline-block h-2 w-2 rounded-full ${
                    wi.severity === "P0"
                      ? "bg-sev-p0"
                      : wi.severity === "P1"
                        ? "bg-sev-p1"
                        : wi.severity === "P2"
                          ? "bg-sev-p2"
                          : "bg-sev-p3"
                  }`}
                  aria-hidden
                />
                <span className="truncate font-mono text-card font-medium text-text-primary">
                  {wi.component_id}
                  <span className="ml-2 font-mono text-meta text-text-tertiary">
                    {wi.id.slice(0, 8)}
                  </span>
                </span>
                <span><SeverityBadge severity={wi.severity} /></span>
                <span><StatePill status={wi.status} /></span>
                <span className="text-right font-mono text-data tabular-nums text-text-secondary">
                  {wi.signal_count.toLocaleString()}
                </span>
                <span className="font-mono text-meta text-text-primary tabular-nums">
                  {fmtMTTR(wi.mttr_seconds)}
                </span>
                <span className="font-mono text-meta text-text-tertiary">
                  {fmtRelative(wi.closed_at)}
                </span>
                <IconChevronRight
                  size={16}
                  className="justify-self-end text-text-tertiary group-hover:text-text-secondary"
                  aria-hidden
                />
              </Link>
            ))
          )}
        </div>
      </main>
    </div>
  );
}
