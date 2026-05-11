"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import { useRef } from "react";

interface CornerTicksProps {

  tone?: "lime" | "muted";

  size?: number;
}

export function CornerTicks({ tone = "lime", size = 14 }: CornerTicksProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-10%" });

  const reduced = !!useReducedMotion();
  const stroke =
    tone === "muted" ? "var(--text-tertiary)" : "var(--accent)";
  const corners: Array<{ pos: string; d: string }> = [
    { pos: "-top-px -left-px", d: `M 0 ${size} L 0 0 L ${size} 0` },
    { pos: "-top-px -right-px", d: `M 0 0 L ${size} 0 L ${size} ${size}` },
    { pos: "-bottom-px -left-px", d: `M 0 0 L 0 ${size} L ${size} ${size}` },
    {
      pos: "-bottom-px -right-px",
      d: `M 0 ${size} L ${size} ${size} L ${size} 0`,
    },
  ];
  return (
    <>
      {}
      <span ref={ref} aria-hidden className="pointer-events-none absolute inset-0" />
      {corners.map((c, i) => (
        <svg
          key={i}
          width={size}
          height={size}
          viewBox={`0 0 ${size} ${size}`}
          fill="none"
          className={`pointer-events-none absolute ${c.pos}`}
          aria-hidden
        >
          <motion.path
            d={c.d}
            stroke={stroke}
            strokeWidth="1.5"
            strokeLinecap="square"
            initial={reduced ? false : { pathLength: 0, opacity: 0 }}
            animate={
              reduced
                ? undefined
                : inView
                  ? { pathLength: 1, opacity: 0.9 }
                  : undefined
            }
            transition={{
              duration: 0.6,
              delay: 0.15 + i * 0.08,
              ease: [0.16, 1, 0.3, 1],
            }}
          />
        </svg>
      ))}
    </>
  );
}
