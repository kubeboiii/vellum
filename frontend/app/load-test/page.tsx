"use client";

import { useRef, useState } from "react";

import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { Nav } from "@/components/Nav";
import { postSignal } from "@/lib/api";
import { APIError, type ComponentType, type Severity } from "@/lib/types";

const COUNT_MAX = 10_000;
const RPS_MAX = 5_000;

const COMPONENT_TYPES: ComponentType[] = [
  "API",
  "MCP_HOST",
  "CACHE",
  "QUEUE",
  "RDBMS",
  "NOSQL",
  "OTHER",
];

const SEVERITIES: Severity[] = ["P0", "P1", "P2", "P3"];

type RunState = "idle" | "running" | "done";

interface RunStats {
  sent: number;
  accepted: number;
  rejected: number;
  failed: number;
  startedAt: number;
  endedAt: number | null;
  latencies: number[];
}

const ZERO_STATS: RunStats = {
  sent: 0,
  accepted: 0,
  rejected: 0,
  failed: 0,
  startedAt: 0,
  endedAt: null,
  latencies: [],
};

export default function LoadTestPage() {
  const [count, setCount] = useState(500);
  const [rps, setRps] = useState(250);
  const [component, setComponent] = useState("LOAD_TEST_API_01");
  const [componentType, setComponentType] = useState<ComponentType>("API");
  const [severity, setSeverity] = useState<Severity>("P2");
  const [state, setState] = useState<RunState>("idle");
  const [stats, setStats] = useState<RunStats>(ZERO_STATS);
  const cancelRef = useRef(false);

  const sendBurst = async () => {
    const safeCount = Math.min(COUNT_MAX, Math.max(1, count));
    const safeRps = Math.min(RPS_MAX, Math.max(1, rps));
    const intervalMs = Math.max(1, 1000 / safeRps);
    cancelRef.current = false;

    const s: RunStats = { ...ZERO_STATS, startedAt: performance.now() };
    setStats(s);
    setState("running");

    const promises: Promise<void>[] = [];
    let lastSent = performance.now();
    for (let i = 0; i < safeCount; i++) {
      if (cancelRef.current) break;
      const t0 = performance.now();
      const p = postSignal({
        component_id: component,
        component_type: componentType,
        severity,
        source: "load-test",
        payload: { i, burst_id: s.startedAt },
      })
        .then(() => {
          const dt = performance.now() - t0;
          setStats((cur) => ({
            ...cur,
            accepted: cur.accepted + 1,
            latencies: cur.latencies.concat(dt),
          }));
        })
        .catch((err) => {
          const dt = performance.now() - t0;
          if (err instanceof APIError && err.status === 503) {
            setStats((cur) => ({
              ...cur,
              rejected: cur.rejected + 1,
              latencies: cur.latencies.concat(dt),
            }));
          } else {
            setStats((cur) => ({
              ...cur,
              failed: cur.failed + 1,
              latencies: cur.latencies.concat(dt),
            }));
          }
        });
      promises.push(p);
      setStats((cur) => ({ ...cur, sent: cur.sent + 1 }));

      const nextAt = lastSent + intervalMs;
      lastSent = nextAt;
      const waitMs = nextAt - performance.now();
      if (waitMs > 0) await new Promise((r) => setTimeout(r, waitMs));
    }
    await Promise.all(promises);
    setStats((cur) => ({ ...cur, endedAt: performance.now() }));
    setState("done");
  };

  const stop = () => {
    cancelRef.current = true;
  };

  const reset = () => {
    cancelRef.current = false;
    setStats(ZERO_STATS);
    setState("idle");
  };

  const durationMs = stats.endedAt ? stats.endedAt - stats.startedAt : null;
  const throughput =
    durationMs && durationMs > 0
      ? (stats.accepted / (durationMs / 1000)).toFixed(0)
      : "—";
  const p99 = pctl(stats.latencies, 0.99).toFixed(0);
  const p50 = pctl(stats.latencies, 0.5).toFixed(0);

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Load test" />
      <HealthStrip />
      <main className="mx-auto max-w-[1000px] space-y-4 px-6 py-4">
        <header>
          <h1 className="relative pb-2 font-sans text-page font-semibold text-text-primary">
            Load test
            <span className="absolute bottom-0 left-0 h-px w-6 bg-accent" aria-hidden />
          </h1>
          <p className="mt-2 font-mono text-meta text-text-tertiary">
            Fires real POSTs to{" "}
            <span className="text-text-secondary">/v1/signals</span>. Counts
            202 vs 503 to show the backpressure handshake live. Capped at{" "}
            {COUNT_MAX.toLocaleString()} signals · {RPS_MAX.toLocaleString()}{" "}
            rps so a misclick doesn&apos;t DoS your own laptop.
          </p>
        </header>

        {}
        <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-5">
            <NumberField label="count" value={count} onChange={setCount} max={COUNT_MAX} />
            <NumberField label="rps" value={rps} onChange={setRps} max={RPS_MAX} />
            <SelectField
              label="severity"
              value={severity}
              options={SEVERITIES}
              onChange={(v) => setSeverity(v as Severity)}
            />
            <SelectField
              label="component type"
              value={componentType}
              options={COMPONENT_TYPES}
              onChange={(v) => setComponentType(v as ComponentType)}
            />
            <TextField label="component id" value={component} onChange={setComponent} />
          </div>
          <div className="mt-3 flex items-center gap-2 border-t border-border-subtle pt-3">
            <button
              type="button"
              onClick={sendBurst}
              disabled={state === "running"}
              className="rounded-sm bg-accent px-4 py-2 font-sans text-meta font-medium text-accent-text transition-[background-color,box-shadow] duration-fast hover:bg-accent-bright hover:shadow-[0_0_18px_-6px_rgba(190,242,100,0.55)] disabled:opacity-50"
            >
              {state === "running" ? "running…" : "Send burst"}
            </button>
            {state === "running" && (
              <button
                type="button"
                onClick={stop}
                className="rounded-sm border border-border-subtle bg-transparent px-4 py-2 font-sans text-meta text-text-primary transition-colors duration-fast hover:bg-bg-elevated"
              >
                Stop
              </button>
            )}
            {state === "done" && (
              <button
                type="button"
                onClick={reset}
                className="rounded-sm border border-border-subtle bg-transparent px-4 py-2 font-sans text-meta text-text-primary transition-colors duration-fast hover:bg-bg-elevated"
              >
                Reset
              </button>
            )}
            <span className="ml-auto inline-flex items-center gap-2 font-mono text-meta uppercase tracking-[0.05em]">
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  state === "running"
                    ? "animate-pulse-live bg-accent"
                    : state === "done"
                      ? "bg-text-tertiary"
                      : "bg-text-tertiary"
                }`}
                aria-hidden
              />
              <span
                className={
                  state === "running"
                    ? "text-accent"
                    : "text-text-tertiary"
                }
              >
                {state}
              </span>
            </span>
          </div>
        </section>

        {}
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Counter label="sent" value={stats.sent} tone="neutral" />
          <Counter label="accepted (202)" value={stats.accepted} tone="good" />
          <Counter label="rejected (503)" value={stats.rejected} tone="bad" />
          <Counter label="failed" value={stats.failed} tone="bad" />
        </section>

        {}
        {state === "done" && (
          <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
            <h2 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
              Summary
            </h2>
            <dl className="mt-3 grid grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-4">
              <Field label="duration" value={`${(durationMs! / 1000).toFixed(2)}s`} />
              <Field label="throughput" value={`${throughput}/s`} />
              <Field label="p50 latency" value={`${p50}ms`} />
              <Field label="p99 latency" value={`${p99}ms`} />
            </dl>
          </section>
        )}
      </main>
    </div>
  );
}

function NumberField({
  label,
  value,
  onChange,
  max,
}: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  max: number;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        {label}
      </span>
      <input
        type="number"
        min={1}
        max={max}
        value={value}
        onChange={(e) => {
          const n = parseInt(e.target.value, 10);
          if (Number.isFinite(n)) onChange(Math.min(max, Math.max(1, n)));
        }}
        className="h-8 rounded-sm border border-border-subtle bg-bg-input px-2 font-mono text-data text-text-primary tabular-nums transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
      />
    </label>
  );
}

function SelectField<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly T[];
  onChange: (v: T) => void;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        {label}
      </span>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value as T)}
        className="h-8 rounded-sm border border-border-subtle bg-bg-input px-2 font-mono text-data text-text-primary transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
      >
        {options.map((o) => (
          <option key={o} value={o}>
            {o}
          </option>
        ))}
      </select>
    </label>
  );
}

function TextField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  return (
    <label className="flex flex-col gap-1">
      <span className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        {label}
      </span>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="h-8 rounded-sm border border-border-subtle bg-bg-input px-2 font-mono text-data text-text-primary transition-[border-color,box-shadow] duration-fast focus-visible:border-border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/45"
      />
    </label>
  );
}

function Counter({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: "neutral" | "good" | "bad";
}) {
  const valueCls =
    tone === "good"
      ? "text-accent"
      : tone === "bad"
        ? "text-sev-p0"
        : "text-text-primary";
  return (
    <div className="rounded-md border border-border-subtle bg-bg-surface p-3">
      <div className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        {label}
      </div>
      <div className={`mt-1 font-mono text-stat font-medium tabular-nums ${valueCls}`}>
        {value.toLocaleString()}
      </div>
    </div>
  );
}

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        {label}
      </dt>
      <dd className="font-mono text-data text-text-primary tabular-nums">{value}</dd>
    </div>
  );
}

function pctl(arr: number[], p: number): number {
  if (arr.length === 0) return 0;
  const sorted = [...arr].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.floor(sorted.length * p));
  return sorted[idx];
}
