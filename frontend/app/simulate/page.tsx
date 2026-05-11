"use client";

import Link from "next/link";
import { useRef, useState } from "react";

import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { Nav } from "@/components/Nav";
import { listIncidents, postSignal } from "@/lib/api";
import { APIError, type ComponentType, type Severity, type WorkItem } from "@/lib/types";

interface ScenarioStep {
  count: number;
  rps: number;
  component_id: string;
  component_type: ComponentType;
  severity: Severity;
}

interface Scenario {
  id: string;
  title: string;
  blurb: string;
  expectedIncidents: string;
  steps: ScenarioStep[];
}

interface RunResult {
  startedAt: number;
  endedAt: number;
  sent: number;
  accepted: number;
  rejected: number;
  failed: number;

  workItems: WorkItem[];
}

function predictedIncidents(steps: ScenarioStep[]): number {

  const byComp = new Map<string, number>();
  for (const s of steps) {
    byComp.set(s.component_id, (byComp.get(s.component_id) ?? 0) + s.count);
  }
  let total = 0;
  for (const total_count of Array.from(byComp.values())) {
    total += Math.ceil(total_count / 100);
  }
  return total;
}

const SCENARIOS: Scenario[] = [
  {
    id: "rdbms-cascade",
    title: "RDBMS cascade",
    blurb:
      "50 P0 signals to RDBMS_PRIMARY_01, then 100 P1 signals to API_CHECKOUT as the cascade hits the dependent service.",
    expectedIncidents: "≈ 2 incidents",
    steps: [
      {
        count: 50,
        rps: 25,
        component_id: "RDBMS_PRIMARY_01",
        component_type: "RDBMS",
        severity: "P0",
      },
      {
        count: 100,
        rps: 50,
        component_id: "API_CHECKOUT",
        component_type: "API",
        severity: "P1",
      },
    ],
  },
  {
    id: "cache-thrash",
    title: "Cache thrash",
    blurb:
      "200 P2 signals to CACHE_CLUSTER_A. Single-component noise — the debouncer caps each window at 100 signals (FR-3.1), so 200 → 2 work items.",
    expectedIncidents: "≈ 2 incidents",
    steps: [
      {
        count: 200,
        rps: 20,
        component_id: "CACHE_CLUSTER_A",
        component_type: "CACHE",
        severity: "P2",
      },
    ],
  },
  {
    id: "mcp-host-fail",
    title: "MCP host fail",
    blurb:
      "30 P0 signals to MCP_HOST_INDEXER + 80 P1 fanned across 4 APIs (20 each). Tests multi-component debounce and severity-mixed alerting.",
    expectedIncidents: "≈ 5 incidents",
    steps: [
      {
        count: 30,
        rps: 15,
        component_id: "MCP_HOST_INDEXER",
        component_type: "MCP_HOST",
        severity: "P0",
      },
      {
        count: 20,
        rps: 20,
        component_id: "API_SEARCH",
        component_type: "API",
        severity: "P1",
      },
      {
        count: 20,
        rps: 20,
        component_id: "API_RECOMMEND",
        component_type: "API",
        severity: "P1",
      },
      {
        count: 20,
        rps: 20,
        component_id: "API_HOMEFEED",
        component_type: "API",
        severity: "P1",
      },
      {
        count: 20,
        rps: 20,
        component_id: "API_NOTIFICATIONS",
        component_type: "API",
        severity: "P1",
      },
    ],
  },
];

export default function SimulatePage() {
  const [runningId, setRunningId] = useState<string | null>(null);
  const [progress, setProgress] = useState<Record<string, number>>({});

  const [live, setLive] = useState<
    Record<string, { sent: number; accepted: number; rejected: number; failed: number }>
  >({});

  const [lastRun, setLastRun] = useState<Record<string, RunResult>>({});
  const cancelRef = useRef(false);

  const runScenario = async (sc: Scenario) => {
    cancelRef.current = false;
    setRunningId(sc.id);
    setProgress((cur) => ({ ...cur, [sc.id]: 0 }));
    setLive((cur) => ({
      ...cur,
      [sc.id]: { sent: 0, accepted: 0, rejected: 0, failed: 0 },
    }));
    setLastRun((cur) => {
      const next = { ...cur };
      delete next[sc.id];
      return next;
    });

    const total = sc.steps.reduce((a, s) => a + s.count, 0);
    const startedAt = performance.now();
    let sent = 0;
    const promises: Promise<void>[] = [];

    for (const step of sc.steps) {
      if (cancelRef.current) break;
      const intervalMs = Math.max(1, 1000 / step.rps);
      let nextAt = performance.now();
      for (let i = 0; i < step.count; i++) {
        if (cancelRef.current) break;

        const p = postSignal({
          component_id: step.component_id,
          component_type: step.component_type,
          severity: step.severity,
          source: "simulate",
          payload: {
            scenario: sc.id,
            step_component: step.component_id,
            i,
          },
        })
          .then(() => {
            setLive((cur) => ({
              ...cur,
              [sc.id]: {
                ...cur[sc.id],
                accepted: cur[sc.id].accepted + 1,
              },
            }));
          })
          .catch((err) => {
            setLive((cur) => ({
              ...cur,
              [sc.id]: {
                ...cur[sc.id],
                ...(err instanceof APIError && err.status === 503
                  ? { rejected: cur[sc.id].rejected + 1 }
                  : { failed: cur[sc.id].failed + 1 }),
              },
            }));
          });
        promises.push(p);
        sent++;
        setProgress((cur) => ({ ...cur, [sc.id]: sent / total }));
        setLive((cur) => ({
          ...cur,
          [sc.id]: { ...cur[sc.id], sent },
        }));
        nextAt += intervalMs;
        const waitMs = nextAt - performance.now();
        if (waitMs > 0) await new Promise((r) => setTimeout(r, waitMs));
      }
    }

    await Promise.allSettled(promises);
    const endedAt = performance.now();

    let workItems: WorkItem[] = [];
    try {
      const components = new Set(sc.steps.map((s) => s.component_id));
      const data = await listIncidents();
      workItems = data.items.filter(
        (wi) =>
          components.has(wi.component_id) &&

          new Date(wi.last_signal_ts).getTime() >= Date.now() - (endedAt - startedAt) - 5000,
      );
    } catch {

    }

    const lcur = (i: string) => live[i] ?? { sent: 0, accepted: 0, rejected: 0, failed: 0 };

    setLive((cur) => {
      const final = cur[sc.id] ?? lcur(sc.id);
      setLastRun((prev) => ({
        ...prev,
        [sc.id]: {
          startedAt,
          endedAt,
          sent: final.sent,
          accepted: final.accepted,
          rejected: final.rejected,
          failed: final.failed,
          workItems,
        },
      }));
      return cur;
    });
    setRunningId(null);
  };

  const stop = () => {
    cancelRef.current = true;
  };

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Simulate" />
      <HealthStrip />
      <main className="mx-auto max-w-[1100px] space-y-4 px-6 py-4">
        <header>
          <h1 className="relative pb-2 font-sans text-page font-semibold text-text-primary">
            Failure simulator
            <span
              className="absolute bottom-0 left-0 h-px w-6 bg-accent"
              aria-hidden
            />
          </h1>
          <p className="mt-2 font-mono text-meta text-text-tertiary">
            Pre-canned scenarios that POST real signals to your local
            backend. Click <em>Run scenario</em>; the card shows what
            landed plus a link into the live feed for what got created.
            You can also open{" "}
            <Link
              href="/dashboard"
              className="text-accent hover:text-accent-bright"
            >
              the dashboard
            </Link>{" "}
            in another tab to watch it happen in real time.
          </p>
        </header>

        <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
          {SCENARIOS.map((sc) => {
            const isRunning = runningId === sc.id;
            const isBusy = runningId !== null && !isRunning;
            const p = progress[sc.id] ?? 0;
            const liveCounters = live[sc.id];
            const completedRun = lastRun[sc.id];
            const predicted = predictedIncidents(sc.steps);
            return (
              <article
                key={sc.id}
                className="flex flex-col rounded-md border border-border-subtle bg-bg-surface p-4"
              >
                <header className="mb-2">
                  <h2 className="font-sans text-card font-semibold text-text-primary">
                    {sc.title}
                  </h2>
                  <p className="mt-1 font-mono text-meta text-accent">
                    {sc.expectedIncidents} ·{" "}
                    <span className="text-text-tertiary">
                      math: ceil(Σcount/100) = {predicted}
                    </span>
                  </p>
                </header>
                <p className="flex-1 font-sans text-body leading-[1.5] text-text-secondary">
                  {sc.blurb}
                </p>

                {}
                <ul className="mt-3 space-y-1">
                  {sc.steps.map((step, i) => (
                    <li
                      key={i}
                      className="flex items-center justify-between gap-2 font-mono text-meta text-text-tertiary"
                    >
                      <span className="truncate text-text-secondary">
                        {step.severity} · {step.component_id}
                      </span>
                      <span className="shrink-0 tabular-nums">
                        {step.count} @ {step.rps}/s
                      </span>
                    </li>
                  ))}
                </ul>

                {}
                {isRunning && liveCounters && (
                  <div className="mt-3 space-y-2">
                    <div className="h-1 overflow-hidden rounded-sm bg-bg-elevated">
                      <div
                        className="h-full bg-accent transition-[width] duration-fast"
                        style={{ width: `${p * 100}%` }}
                      />
                    </div>
                    <div className="flex items-center justify-between font-mono text-meta tabular-nums">
                      <span className="text-text-secondary">
                        sent{" "}
                        <span className="text-text-primary">{liveCounters.sent}</span>
                      </span>
                      <span className="text-accent">
                        ✓ {liveCounters.accepted}
                      </span>
                      <span className="text-sev-p1">
                        503 {liveCounters.rejected}
                      </span>
                      <span className="text-sev-p0">
                        ✗ {liveCounters.failed}
                      </span>
                    </div>
                  </div>
                )}

                {}
                {!isRunning && completedRun && (
                  <ResultCard run={completedRun} predicted={predicted} />
                )}

                <div className="mt-3 flex items-center gap-2 border-t border-border-subtle pt-3">
                  <button
                    type="button"
                    onClick={() => runScenario(sc)}
                    disabled={isBusy || isRunning}
                    className="rounded-sm bg-accent px-3 py-1.5 font-sans text-meta font-medium text-accent-text transition-[background-color,box-shadow] duration-fast hover:bg-accent-bright hover:shadow-[0_0_18px_-6px_rgba(190,242,100,0.55)] disabled:opacity-50"
                  >
                    {isRunning
                      ? "running…"
                      : completedRun
                        ? "Run again"
                        : "Run scenario"}
                  </button>
                  {isRunning && (
                    <button
                      type="button"
                      onClick={stop}
                      className="rounded-sm border border-border-subtle bg-transparent px-3 py-1.5 font-sans text-meta text-text-primary transition-colors duration-fast hover:bg-bg-elevated"
                    >
                      Stop
                    </button>
                  )}
                </div>
              </article>
            );
          })}
        </div>
      </main>
    </div>
  );
}

function ResultCard({
  run,
  predicted,
}: {
  run: RunResult;
  predicted: number;
}) {
  const durSec = ((run.endedAt - run.startedAt) / 1000).toFixed(2);
  const rate = run.sent / Math.max(0.001, (run.endedAt - run.startedAt) / 1000);
  return (
    <div className="mt-3 space-y-2 rounded-sm border border-border-subtle bg-bg-elevated p-2">
      {}
      <div className="flex items-center justify-between font-mono text-meta tabular-nums">
        <span className="text-text-secondary">
          sent <span className="text-text-primary">{run.sent}</span>
        </span>
        <span className="text-accent" title="202 accepted">
          ✓ {run.accepted}
        </span>
        <span className="text-sev-p1" title="503 backpressure rejected">
          503 {run.rejected}
        </span>
        <span className="text-sev-p0" title="other failure">
          ✗ {run.failed}
        </span>
      </div>
      <div className="flex items-center justify-between font-mono text-meta tabular-nums text-text-tertiary">
        <span>{durSec}s · {rate.toFixed(0)}/s actual</span>
        <span>
          debouncer · {run.workItems.length}/{predicted} predicted
        </span>
      </div>

      {}
      {run.workItems.length > 0 && (
        <ul className="space-y-0.5 border-t border-border-subtle pt-2">
          {run.workItems.slice(0, 5).map((wi) => (
            <li
              key={wi.id}
              className="flex items-center gap-2 font-mono text-meta"
            >
              <span
                className={`h-1 w-1 rounded-full ${sevDot(wi.severity)}`}
                aria-hidden
              />
              <Link
                href={`/incidents/${wi.id}`}
                className="truncate text-text-primary hover:text-accent"
              >
                {wi.component_id}
              </Link>
              <span className="ml-auto shrink-0 text-text-tertiary tabular-nums">
                {wi.signal_count}×
              </span>
            </li>
          ))}
          {run.workItems.length > 5 && (
            <li className="font-mono text-meta text-text-tertiary">
              + {run.workItems.length - 5} more…
            </li>
          )}
        </ul>
      )}
      {run.workItems.length === 0 && run.accepted > 0 && (
        <p className="border-t border-border-subtle pt-2 font-mono text-meta text-text-tertiary">
          Signals accepted but no matching incidents found yet — the
          dashboard&apos;s next poll (within 2s) will catch them.{" "}
          <Link
            href="/dashboard"
            className="text-accent hover:text-accent-bright"
          >
            Open live feed ›
          </Link>
        </p>
      )}
    </div>
  );
}

function sevDot(sev: Severity): string {
  return sev === "P0"
    ? "bg-sev-p0"
    : sev === "P1"
      ? "bg-sev-p1"
      : sev === "P2"
        ? "bg-sev-p2"
        : "bg-sev-p3";
}
