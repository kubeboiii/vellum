"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";

type Accent = "lime" | "red" | "violet";

interface Tile {

  numeric?: number;
  prefix?: string;
  suffix?: string;
  staticNumber?: string;
  label: string;
  description: string;
  accent: Accent;
}

const TILES: Tile[] = [
  {
    numeric: 10,
    suffix: "K /sec",
    label: "Ingestion",
    description: "Bounded channel + Go workers absorb bursts without crashing.",
    accent: "lime",
  },
  {
    staticNumber: "100 → 1",
    label: "Debounce ratio",
    description: "One Redis Lua script collapses correlated signals per component.",
    accent: "violet",
  },
  {
    numeric: 50,
    prefix: "<",
    suffix: "ms",
    label: "p99 latency",
    description: "Non-blocking handler. Persistence runs behind a queue.",
    accent: "lime",
  },
  {
    staticNumber: "4 stores",
    label: "Polyglot persistence",
    description: "Postgres, Mongo, Redis, TimescaleDB — each used where it fits.",
    accent: "violet",
  },
  {
    numeric: 0,
    suffix: " dropped",
    label: "Dead-letter recovery",
    description: "Every failed write is captured for human inspection, never silently lost.",
    accent: "red",
  },
  {
    numeric: 100,
    suffix: "%",
    label: "RCA coverage",
    description: "CLOSED is impossible without a complete RCA. MTTR is automatic.",
    accent: "lime",
  },
];

const ACCENT_TOKEN: Record<
  Accent,
  { dot: string; number: string; glow: string; scan: string }
> = {
  lime: {
    dot: "bg-accent",
    number: "text-text-primary",
    glow: "hover:shadow-[0_0_28px_-10px_var(--accent-glow)]",
    scan: "bg-accent",
  },
  red: {
    dot: "bg-sev-p0",
    number: "text-text-primary",
    glow: "hover:shadow-[0_0_28px_-10px_rgba(239,68,68,0.45)]",
    scan: "bg-sev-p0",
  },
  violet: {
    dot: "bg-annotation",
    number: "text-text-primary",
    glow: "hover:shadow-[0_0_28px_-10px_rgba(167,139,250,0.45)]",
    scan: "bg-annotation",
  },
};

function CountUpNumber({
  target,
  prefix = "",
  suffix = "",
  inView,
}: {
  target: number;
  prefix?: string;
  suffix?: string;
  inView: boolean;
}) {
  const reduced = useReducedMotion();
  const [val, setVal] = useState(reduced || !inView ? target : 0);

  useEffect(() => {
    if (reduced || !inView) {
      setVal(target);
      return;
    }
    let raf = 0;
    const start = performance.now();
    const duration = 900;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);

      const eased = 1 - Math.pow(1 - t, 4);
      setVal(Math.round(target * eased));
      if (t < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [target, reduced, inView]);

  return (
    <>
      {prefix}
      {val.toLocaleString()}
      {suffix}
    </>
  );
}

function TileEl({ tile }: { tile: Tile }) {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15%" });
  const a = ACCENT_TOKEN[tile.accent];
  return (
    <div
      ref={ref}
      className={`group relative flex min-h-[160px] flex-col bg-bg-surface px-6 py-7 transition-all duration-base ease-out ${a.glow}`}
    >
      {}
      <div
        className={`font-mono text-[32px] font-medium leading-[1.1] tracking-[-0.01em] tabular-nums ${a.number}`}
      >
        {tile.staticNumber ? (
          tile.staticNumber
        ) : (
          <CountUpNumber
            target={tile.numeric ?? 0}
            prefix={tile.prefix}
            suffix={tile.suffix}
            inView={inView}
          />
        )}
      </div>

      {}
      <motion.div
        className={`mt-2 h-px ${a.scan}`}
        initial={{ width: 0, opacity: 0 }}
        animate={inView ? { width: 24, opacity: 0.7 } : undefined}
        transition={{ duration: 0.6, delay: 0.2, ease: [0.16, 1, 0.3, 1] }}
      />

      {}
      <div className="mt-3 flex items-center gap-2">
        <span className={`h-1.5 w-1.5 rounded-full ${a.dot}`} aria-hidden />
        <span className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          {tile.label}
        </span>
      </div>

      <p className="mt-4 font-sans text-body leading-[1.55] text-text-tertiary">
        {tile.description}
      </p>
    </div>
  );
}

export function Capabilities() {
  return (
    <section className="border-t border-divider px-6 py-24 sm:py-32">
      <div className="mx-auto max-w-[1120px]">
        <header className="mb-12">
          <p className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            What it does
          </p>
          <h2 className="mt-3 font-sans text-[28px] font-medium leading-[1.2] text-text-primary sm:text-[36px]">
            Six numbers, no marketing.
          </h2>
        </header>

        <div className="grid grid-cols-1 gap-px overflow-hidden rounded-md border border-border-subtle bg-border-subtle sm:grid-cols-2 lg:grid-cols-3">
          {TILES.map((t) => (
            <TileEl key={t.label} tile={t} />
          ))}
        </div>
      </div>
    </section>
  );
}
