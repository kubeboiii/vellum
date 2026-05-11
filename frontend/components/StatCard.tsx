"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";
import { Line, LineChart, ResponsiveContainer } from "recharts";

interface StatCardProps {
  label: string;
  value: string | number;
  delta?: string;

  deltaTone?: "neutral" | "good" | "bad";
  sparkline?: number[];
  sparkColor?: string;
}

const deltaToneClass: Record<NonNullable<StatCardProps["deltaTone"]>, string> = {
  neutral: "text-text-tertiary",
  good: "text-accent",
  bad: "text-sev-p0",
};

function CountUpValue({
  value,
  inView,
}: {
  value: string | number;
  inView: boolean;
}) {
  const reduced = useReducedMotion();
  const text = String(value);

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
  sparkColor = "#BEF264",
}: StatCardProps) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10%" });

  const hoverGlow = `0 0 28px -10px ${sparkColor}66`;

  return (
    <motion.div
      ref={ref}
      whileHover={{ y: -1 }}
      transition={{ duration: 0.15 }}
      className="group rounded-md border border-border-subtle bg-bg-surface p-4 transition-[border-color,box-shadow] duration-base ease-out hover:border-border-strong"
      style={
        {

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
