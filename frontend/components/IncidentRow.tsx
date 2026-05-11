// THEME.md §6.3 — Incident row (live feed).
//
// Layout per the spec:
//   [●] CACHE_CLUSTER_01   P0   OPEN          243 signals    08:42:11   2m ago   →
//
// grid-cols-[12px_1fr_50px_120px_120px_100px_90px_20px], gap 12px,
// padding 8px 16px, height 32px. Hover: bg-elevated. Click navigates
// to /incidents/[id]. Pulse the dot if P0+OPEN.

"use client";

import { IconChevronRight } from "@tabler/icons-react";
import { motion } from "framer-motion";
import Link from "next/link";

import { SeverityBadge } from "@/components/SeverityBadge";
import { StatePill } from "@/components/StatePill";
import type { WorkItem } from "@/lib/types";

// shortId returns the first 8 chars of a UUID — §8.4 forbids
// rendering long UUIDs verbatim in the UI.
function shortId(id: string): string {
  return id.slice(0, 8);
}

// relativeTime converts an ISO timestamp into a compact relative
// string ("2m ago", "1h ago"). Pure client-side; no Intl heavy
// dependency. Mirrors Linear's compact time format.
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

// Per-severity hover glow color (matches THEME.md severity scale).
// We inline the shadow on mouseenter because Tailwind can't accept
// dynamic hex values at the hover utility level. ~28% alpha keeps
// the halo subtle.
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
      // §5.2 new-incident fade+slide-down (300ms ease-out). The
      // `layout` prop also smoothly re-orders when severity sort
      // changes (a P0 climbs to the top as it accumulates signals).
      initial={{ opacity: 0, y: -8 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, height: 0, marginTop: 0 }}
      transition={{ duration: 0.3, ease: [0.16, 1, 0.3, 1] }}
      // Severity-keyed hover glow — matches the landing page's
      // hover-glow pattern on cards (LogTape, Capabilities). Halo
      // intensifies with severity: P0 burns red, P3 fades to grey.
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
      // text-data on the row baseline; specific cells override for label/meta.
    >
      {/* Severity dot — same color as P0/P1/P2/P3 but tiny.
          Pulses for P0+OPEN per §6.7. */}
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

      {/* Component ID — mono, the most prominent text in the row. */}
      <span className="truncate font-mono text-card font-medium text-text-primary">
        {wi.component_id}
        <span className="ml-2 font-mono text-meta text-text-tertiary">
          {shortId(wi.id)}
        </span>
      </span>

      {/* Severity badge in its 50px column. */}
      <span><SeverityBadge severity={wi.severity} /></span>

      {/* State pill — workhorse. */}
      <span><StatePill status={wi.status} pulseDot={pulse} /></span>

      {/* Signal count, right-aligned, mono, tabular. */}
      <span className="text-right font-mono text-data tabular-nums text-text-secondary">
        {wi.signal_count.toLocaleString()}{" "}
        <span className="text-text-tertiary">signals</span>
      </span>

      {/* Absolute time. */}
      <span className="font-mono text-meta text-text-tertiary">
        {timeOfDay(wi.last_signal_ts)}
      </span>

      {/* Relative time. */}
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
