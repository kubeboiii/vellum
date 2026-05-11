// /flow — animated end-to-end signal flow diagram. Reuses the
// language of the landing's HowItWorks but tells one story across
// four stages: ingest → debounce → persist (parallel) → alert.
//
// Auto-loops with a 4-second cycle; pause toggle is provided.
// Reduced-motion: render final state, no animation.

"use client";

import { motion, useReducedMotion } from "framer-motion";
import { useEffect, useState } from "react";

import { HealthStrip } from "@/components/dashboard/HealthStrip";
import { Nav } from "@/components/Nav";

const STAGES = ["ingest", "debounce", "persist", "alert"] as const;
type Stage = (typeof STAGES)[number];

const STAGE_LABEL: Record<Stage, string> = {
  ingest: "Ingest",
  debounce: "Debounce",
  persist: "Persist",
  alert: "Alert",
};

const STAGE_EXPLAIN: Record<Stage, string> = {
  ingest:
    "Signal lands at HTTP /v1/signals (or gRPC IngestSignals). Token-bucket rate limiter accepts the source IP. The handler does a NON-BLOCKING channel send and returns 202. Hot path stays under 50ms p99.",
  debounce:
    "A worker picks the signal off the bounded channel and runs one atomic Redis Lua script: GET window for component_id, if active and count<100 → join existing work_item; else → create. Single round-trip. No race even with multiple replicas sharing Redis.",
  persist:
    "Three writes fan out in parallel: Mongo (raw audit), Postgres (work_item update or insert), TimescaleDB (timeseries). Each writer has exponential-backoff retry; final failures dead-letter to Mongo for human inspection.",
  alert:
    "Strategy registry selects an alerter by (component_type, severity). PagerDutyStub for P0, SlackWebhook for P1/P2, ConsoleAlerter as fallback. Dispatch is async and isolated — a failing alerter cannot block ingestion.",
};

export default function FlowPage() {
  const reduced = useReducedMotion();
  const [stage, setStage] = useState<Stage>("ingest");
  const [paused, setPaused] = useState(reduced ?? false);

  // Auto-advance every 4s when not paused.
  useEffect(() => {
    if (paused || reduced) return;
    const id = setInterval(() => {
      setStage((s) => {
        const i = STAGES.indexOf(s);
        return STAGES[(i + 1) % STAGES.length];
      });
    }, 4000);
    return () => clearInterval(id);
  }, [paused, reduced]);

  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <Nav title="Signal flow" />
      <HealthStrip />
      <main className="mx-auto max-w-[1100px] space-y-6 px-6 py-6">
        <header>
          <h1 className="relative pb-2 font-sans text-page font-semibold text-text-primary">
            How a signal flows
            <span className="absolute bottom-0 left-0 h-px w-6 bg-accent" aria-hidden />
          </h1>
          <p className="mt-2 font-mono text-meta text-text-tertiary">
            One signal, end to end. Animation auto-loops; click a stage to
            jump.
          </p>
        </header>

        {/* Stage selector. */}
        <nav
          className="flex flex-wrap items-center gap-2"
          aria-label="stage selector"
        >
          {STAGES.map((s, i) => {
            const active = s === stage;
            return (
              <button
                key={s}
                type="button"
                onClick={() => {
                  setPaused(true);
                  setStage(s);
                }}
                className={`rounded-sm border px-3 py-1 font-mono text-meta uppercase tracking-[0.05em] transition-colors duration-fast ${
                  active
                    ? "border-accent bg-accent-bg text-accent"
                    : "border-border-subtle text-text-tertiary hover:text-text-secondary"
                }`}
              >
                <span className="tabular-nums">0{i + 1}</span>{" "}
                <span className="ml-1">{STAGE_LABEL[s]}</span>
              </button>
            );
          })}
          <button
            type="button"
            onClick={() => setPaused((p) => !p)}
            className="ml-auto rounded-sm border border-border-subtle px-3 py-1 font-mono text-meta uppercase tracking-[0.05em] text-text-primary transition-colors duration-fast hover:bg-bg-elevated"
          >
            {paused ? "▶ Play" : "⏸ Pause"}
          </button>
        </nav>

        {/* Diagram surface. */}
        <section className="relative overflow-hidden rounded-md border border-border-subtle bg-bg-surface p-8">
          <FlowDiagram stage={stage} reduced={!!reduced} />
        </section>

        {/* Explanation strip — tied to active stage. */}
        <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
          <h2 className="flex items-center gap-2 font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            <span
              className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
              aria-hidden
            />
            Stage {STAGES.indexOf(stage) + 1} · {STAGE_LABEL[stage]}
          </h2>
          <p className="mt-2 max-w-[80ch] font-sans text-body leading-[1.55] text-text-secondary">
            {STAGE_EXPLAIN[stage]}
          </p>
        </section>
      </main>
    </div>
  );
}

// FlowDiagram — five nodes connected by arrows. Each node lights
// up when its stage is active. The "current" arrow gets a moving
// dot to suggest the signal's progress.
function FlowDiagram({
  stage,
  reduced,
}: {
  stage: Stage;
  reduced: boolean;
}) {
  // Layout coordinates inside a 1000×220 viewBox.
  return (
    <div className="overflow-x-auto">
      <svg
        viewBox="0 0 1000 220"
        className="h-[220px] w-full min-w-[800px]"
        role="img"
        aria-label="signal flow diagram"
      >
        {/* Background hairline grid (matches landing's GridTexture
            idiom but static here). */}
        <defs>
          <pattern id="flow-grid" width="32" height="32" patternUnits="userSpaceOnUse">
            <path d="M 32 0 L 0 0 0 32" fill="none" stroke="rgba(255,255,255,0.025)" strokeWidth="1" />
          </pattern>
        </defs>
        <rect x="0" y="0" width="1000" height="220" fill="url(#flow-grid)" />

        {/* Nodes */}
        <Node x={60} y={110} label="HTTP / gRPC" sub="POST /v1/signals" active={stage === "ingest"} />
        <Node x={260} y={110} label="Queue" sub="bounded channel" active={stage === "ingest"} />
        <Node x={460} y={110} label="Redis Lua" sub="atomic debounce" active={stage === "debounce"} />
        {/* Three parallel persist sinks */}
        <Node x={680} y={40} label="MongoDB" sub="raw audit" active={stage === "persist"} small />
        <Node x={680} y={110} label="Postgres" sub="work_items" active={stage === "persist"} small />
        <Node x={680} y={180} label="Timescale" sub="signal_metrics" active={stage === "persist"} small />
        <Node x={900} y={110} label="Alerter" sub="Strategy fanout" active={stage === "alert"} />

        {/* Arrows */}
        <Arrow x1={108} y1={110} x2={228} y2={110} active={stage === "ingest"} reduced={reduced} />
        <Arrow x1={308} y1={110} x2={428} y2={110} active={stage === "debounce" || stage === "ingest"} reduced={reduced} />
        <Arrow x1={508} y1={100} x2={648} y2={50} active={stage === "persist"} reduced={reduced} />
        <Arrow x1={508} y1={110} x2={648} y2={110} active={stage === "persist"} reduced={reduced} />
        <Arrow x1={508} y1={120} x2={648} y2={180} active={stage === "persist"} reduced={reduced} />
        <Arrow x1={728} y1={110} x2={868} y2={110} active={stage === "alert"} reduced={reduced} />
      </svg>
    </div>
  );
}

function Node({
  x,
  y,
  label,
  sub,
  active,
  small,
}: {
  x: number;
  y: number;
  label: string;
  sub: string;
  active: boolean;
  small?: boolean;
}) {
  const w = small ? 96 : 112;
  const h = small ? 42 : 54;
  return (
    <g
      transform={`translate(${x - w / 2} ${y - h / 2})`}
      style={{
        filter: active
          ? "drop-shadow(0 0 12px rgba(190,242,100,0.5))"
          : undefined,
      }}
    >
      <rect
        x={0}
        y={0}
        width={w}
        height={h}
        rx={3}
        fill="#0A0A0A"
        stroke={active ? "var(--accent)" : "#2A2A2A"}
        strokeWidth={1.25}
      />
      <text
        x={w / 2}
        y={h / 2 - (small ? 1 : 3)}
        textAnchor="middle"
        fontFamily="ui-sans-serif, system-ui"
        fontSize={small ? 10 : 11}
        fontWeight="500"
        fill={active ? "var(--accent)" : "#D4D4D8"}
      >
        {label}
      </text>
      <text
        x={w / 2}
        y={h / 2 + (small ? 11 : 13)}
        textAnchor="middle"
        fontFamily="ui-monospace, monospace"
        fontSize={small ? 9 : 10}
        fill="#71717A"
      >
        {sub}
      </text>
    </g>
  );
}

function Arrow({
  x1,
  y1,
  x2,
  y2,
  active,
  reduced,
}: {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  active: boolean;
  reduced: boolean;
}) {
  const stroke = active ? "var(--accent)" : "#3F3F46";
  return (
    <g>
      <line
        x1={x1}
        y1={y1}
        x2={x2}
        y2={y2}
        stroke={stroke}
        strokeWidth={active ? 1.5 : 1}
        strokeOpacity={active ? 0.9 : 0.5}
      />
      {/* Arrowhead */}
      <polygon
        points={`${x2},${y2} ${x2 - 6},${y2 - 3} ${x2 - 6},${y2 + 3}`}
        fill={stroke}
        opacity={active ? 0.9 : 0.5}
      />
      {/* Traveling dot for the active arrow. */}
      {active && !reduced && (
        <motion.circle
          r={3}
          fill="var(--accent)"
          initial={{ cx: x1, cy: y1 }}
          animate={{ cx: x2, cy: y2 }}
          transition={{ duration: 1.6, repeat: Infinity, ease: "easeInOut" }}
        />
      )}
    </g>
  );
}
