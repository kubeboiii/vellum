// THEME.md §6.4 — Stat card with sparkline.
//
// Layout:
//   ┌───────────────────────────────┐
//   │ • ACTIVE INCIDENTS            │  ← label + accent dot
//   │ 7                          ▁▃▆ │  ← number 22px mono + sparkline
//   │ +2 from prev                  │  ← delta, mono meta, tone-keyed
//   └───────────────────────────────┘
//
// Aligned with the landing-page design language (Phase 5 polish):
//   * Hover glow color-keyed to the card's accent (defaults to lime,
//     callers can pass a sparkColor to drive the glow tone)
//   * Count-up animation on first reveal — the number ticks from 0
//     to its final value with easeOutQuart over 800ms. Strings that
//     start with a digit (e.g. "8421/s", "12m") animate the digit
//     portion; strings without a leading digit (e.g. "—") render
//     static.
//   * Animated colored dot next to the label, matching sparkColor
//
// All motion gated on framer-motion's useReducedMotion.

"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";
import { Line, LineChart, ResponsiveContainer } from "recharts";

interface StatCardProps {
  label: string;
  value: string | number;
  delta?: string;
  // deltaTone semantically classifies the delta so the right color
  // is used. "up" can mean good (lime — INGEST RATE) or bad (red —
  // P0 count) depending on the metric; the caller decides.
  deltaTone?: "neutral" | "good" | "bad";
  sparkline?: number[];
  sparkColor?: string;
}

const deltaToneClass: Record<NonNullable<StatCardProps["deltaTone"]>, string> = {
  neutral: "text-text-tertiary",
  good: "text-accent",
  bad: "text-sev-p0",
};

// CountUpValue animates a string-with-a-leading-number from 0 → target
// when the parent scrolls into view. Strings without a leading
// numeric component render as-is. Honors prefers-reduced-motion.
function CountUpValue({
  value,
  inView,
}: {
  value: string | number;
  inView: boolean;
}) {
  const reduced = useReducedMotion();
  const text = String(value);

  // Strip a leading integer (with optional K/M suffix). Anything
  // else (e.g. "—", "12m", "8421/s") still works because the regex
  // captures "12" out of "12m" and "8421" out of "8421/s".
  const match = text.match(/^(\d+)([KM]?)(.*)$/);
  const target = match ? parseInt(match[1], 10) : null;
  const suffix = match ? `${match[2]}${match[3]}` : "";

  const [display, setDisplay] = useState<string>(
    target !== null && !reduced && !inView ? `0${suffix}` : text,
  );

  useEffect(() => {
    if (target === null || reduced || !inView) {
      setDisplay(text);
      return;
    }
    let raf = 0;
    const start = performance.now();
    const duration = 800;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(1 - t, 4);
      const n = Math.round(target * eased);
      setDisplay(`${n.toLocaleString()}${suffix}`);
      if (t < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [target, suffix, text, reduced, inView]);

  return <>{display}</>;
}

export function StatCard({
  label,
  value,
  delta,
  deltaTone = "neutral",
  sparkline,
  sparkColor = "#BEF264", // default to lime
}: StatCardProps) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10%" });

  // Hover glow uses the sparkColor as the halo. We can't put a raw
  // hex value in a Tailwind hover class, so we build the shadow
  // inline. This matches the landing page's "hover-glow" pattern.
  const hoverGlow = `0 0 28px -10px ${sparkColor}66`; // ~40% alpha

  return (
    <motion.div
      ref={ref}
      whileHover={{ y: -1 }}
      transition={{ duration: 0.15 }}
      className="group rounded-md border border-border-subtle bg-bg-surface p-4 transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
      style={
        {
          // Drive the hover glow via a custom property so we can
          // reference it in the Tailwind className above (or just
          // inline). We inline it on the hover state via a style
          // tag below — but the cleaner approach is the group-hover
          // trick: paint the shadow always at 0 alpha unless hovered.
        } as React.CSSProperties
      }
      onMouseEnter={(e) => {
        e.currentTarget.style.boxShadow = hoverGlow;
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.boxShadow = "";
      }}
    >
      <div className="flex items-center gap-1.5 text-label uppercase tracking-[0.05em] text-text-secondary">
        <span
          className="h-1 w-1 animate-pulse-live rounded-full"
          style={{ backgroundColor: sparkColor }}
          aria-hidden
        />
        {label}
      </div>
      <div className="mt-1 flex items-end justify-between gap-3">
        <div className="font-mono text-stat font-medium text-text-primary tabular-nums">
          <CountUpValue value={value} inView={inView} />
        </div>
        {sparkline && sparkline.length > 1 && (
          <div className="h-4 w-[60px]">
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={sparkline.map((y, i) => ({ i, y }))}>
                <Line
                  type="monotone"
                  dataKey="y"
                  stroke={sparkColor}
                  strokeWidth={1}
                  dot={false}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          </div>
        )}
      </div>
      {delta && (
        <div className={`mt-1 font-mono text-meta ${deltaToneClass[deltaTone]}`}>
          {delta}
        </div>
      )}
    </motion.div>
  );
}
