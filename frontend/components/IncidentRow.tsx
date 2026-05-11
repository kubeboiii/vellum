"use client";

import { IconChevronRight } from "@tabler/icons-react";
import { motion } from "framer-motion";
import Link from "next/link";

import { SeverityBadge } from "@/components/SeverityBadge";
import { StatePill } from "@/components/StatePill";
import type { WorkItem } from "@/lib/types";

function shortId(id: string): string {
  return id.slice(0, 8);
}

function relativeTime(iso: string): string {
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

function timeOfDay(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleTimeString([], { hour12: false });
}

interface IncidentRowProps {
  wi: WorkItem;
}

const SEV_GLOW: Record<WorkItem["severity"], string> = {
  P0: "0 0 24px -10px rgba(239,68,68,0.55)",
  P1: "0 0 24px -10px rgba(245,158,11,0.45)",
  P2: "0 0 24px -10px rgba(190,242,100,0.40)",
  P3: "0 0 24px -10px rgba(113,113,122,0.40)",
};

export function IncidentRow({ wi }: IncidentRowProps) {
  const pulse = wi.severity === "P0" && wi.status === "OPEN";

  return (
    <motion.div
      layout

      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, height: 0, marginTop: 0 }}
      transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}

      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = SEV_GLOW[wi.severity];
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "";
      }}
      style={{ transition: "box-shadow 150ms ease-out" }}
    >
    <Link
      href={`/incidents/${wi.id}`}
      className="group grid h-8 grid-cols-[12px_1fr_56px_140px_120px_100px_90px_20px] items-center gap-3 border-b border-border-subtle px-4 transition-colors duration-fast ease-out hover:bg-bg-hover focus-visible:bg-bg-hover focus-visible:outline-none"

    >
      {}
      <span
        className={`inline-block h-2 w-2 rounded-full ${
          wi.severity === "P0"
            ? "bg-sev-p0"
            : wi.severity === "P1"
              ? "bg-sev-p1"
              : wi.severity === "P2"
                ? "bg-sev-p2"
                : "bg-sev-p3"
        } ${pulse ? "animate-pulse-p0" : ""}`}
        aria-hidden
      />

      {}
      <span className="truncate font-mono text-card font-medium text-text-primary">
        {wi.component_id}
        <span className="ml-2 font-mono text-meta text-text-tertiary">
          {shortId(wi.id)}
        </span>
      </span>

      {}
      <span><SeverityBadge severity={wi.severity} /></span>

      {}
      <span><StatePill status={wi.status} pulseDot={pulse} /></span>

      {}
      <span className="text-right font-mono text-data tabular-nums text-text-secondary">
        {wi.signal_count.toLocaleString()}{" "}
        <span className="text-text-tertiary">signals</span>
      </span>

      {}
      <span className="font-mono text-meta text-text-tertiary">
        {timeOfDay(wi.last_signal_ts)}
      </span>

      {}
      <span className="font-mono text-meta text-text-secondary">
        {relativeTime(wi.last_signal_ts)}
      </span>

      <IconChevronRight
        size={16}
        className="justify-self-end text-text-tertiary group-hover:text-text-secondary"
        aria-hidden
      />
    </Link>
    </motion.div>
  );
}
