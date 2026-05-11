// HealthStrip — polls /health every 5s and renders one chip per
// dependency (Postgres / Mongo / Redis / Timescale) plus a queue
// depth gauge. Lives under the Nav on dashboard routes so the
// operator can spot a degraded dep before clicking into anything.
//
// Resilience: if /health is unreachable (backend offline), render
// a single "backend offline" chip instead of crashing.

"use client";

import { useEffect, useState } from "react";

import { getHealth } from "@/lib/api";
import type { DepStatus, Health } from "@/lib/types";

import { QueueGauge } from "./QueueGauge";

const POLL_MS = 5000;

const DEP_LABELS: Record<string, string> = {
  postgres: "Postgres",
  mongo: "Mongo",
  redis: "Redis",
  timescale: "Timescale",
};

// The backend uses "up" on dep status and "healthy" on the roll-up;
// the spec calls them "ok". Map both to the same UI tone so the
// HealthStrip doesn't care which vocabulary the backend speaks.
function normalize(s: DepStatus | string | undefined): "ok" | "degraded" | "down" {
  if (s === "ok" || s === "up" || s === "healthy") return "ok";
  if (s === "degraded") return "degraded";
  return "down";
}

const DOT_COLOR: Record<"ok" | "degraded" | "down", string> = {
  ok: "bg-accent",
  degraded: "bg-sev-p1",
  down: "bg-sev-p0",
};

export function HealthStrip() {
  const [health, setHealth] = useState<Health | null>(null);
  const [offline, setOffline] = useState(false);

  useEffect(() => {
    let cancelled = false;
    async function tick() {
      try {
        const h = await getHealth();
        if (!cancelled) {
          setHealth(h);
          setOffline(false);
        }
      } catch {
        if (!cancelled) setOffline(true);
      }
    }
    tick();
    const id = setInterval(tick, POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  if (offline) {
    return (
      <div
        className="flex h-7 items-center gap-2 border-b border-border-subtle bg-bg-elevated px-6"
        aria-label="backend health"
      >
        <span
          className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-sev-p0"
          aria-hidden
        />
        <span className="font-mono text-meta uppercase tracking-[0.05em] text-sev-p0">
          Backend offline
        </span>
        <span className="font-mono text-meta text-text-tertiary">
          · /health is unreachable. Polling resumes when the backend returns.
        </span>
      </div>
    );
  }
  if (!health) {
    // First-tick skeleton — keep the height stable so the page
    // doesn't reflow when the first poll lands.
    return (
      <div className="flex h-7 items-center gap-2 border-b border-border-subtle bg-bg-elevated px-6">
        <span className="font-mono text-meta uppercase tracking-[0.05em] text-text-tertiary">
          checking health…
        </span>
      </div>
    );
  }

  // Iterate over the deps the backend actually returned, not a
  // fixed list. Defends against v1 backend (no timescale) while
  // still rendering nicely once it's added.
  const depKeys = Object.keys(health.dependencies ?? {});

  return (
    <div
      className="flex h-7 flex-wrap items-center gap-x-4 gap-y-1 border-b border-border-subtle bg-bg-elevated px-6"
      aria-label="backend health"
    >
      {depKeys.map((k) => {
        const dep = health.dependencies[k as keyof typeof health.dependencies];
        if (!dep) return null;
        return (
          <DepChip
            key={k}
            label={DEP_LABELS[k] ?? k}
            status={dep.status}
            latency={dep.latency_ms}
          />
        );
      })}
      <span
        className="h-3 w-px bg-border-subtle"
        aria-hidden
      />
      <QueueGauge depth={health.queue_depth} capacity={health.queue_capacity} />
      <span className="ml-auto font-mono text-meta text-text-tertiary tabular-nums">
        uptime {fmtUptime(health.uptime_seconds)}
      </span>
    </div>
  );
}

function DepChip({
  label,
  status,
  latency,
}: {
  label: string;
  status: DepStatus;
  latency: number;
}) {
  const tone = normalize(status);
  return (
    <span
      className="inline-flex items-center gap-1.5 font-mono text-meta uppercase tracking-[0.05em] text-text-secondary"
      title={`${label} ${status} · ${latency}ms`}
    >
      <span
        className={`h-1.5 w-1.5 rounded-full ${DOT_COLOR[tone]} ${
          tone === "down" ? "animate-pulse-live" : ""
        }`}
        aria-hidden
      />
      {label}
      <span className="text-text-tertiary tabular-nums">{latency}ms</span>
    </span>
  );
}

function fmtUptime(s: number): string {
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${m % 60}m`;
  return `${Math.floor(h / 24)}d ${h % 24}h`;
}
