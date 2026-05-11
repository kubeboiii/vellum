"use client";

import { AnimatePresence, motion, useReducedMotion } from "framer-motion";
import { useEffect, useState } from "react";

interface Row {

  id: number;
  ts: string;
  accepted: number;
  processed: number;
  queue: number;
}

let nextId = 0;

function pad(n: number | string, width: number): string {
  const s = String(n);
  return s.length >= width ? s : " ".repeat(width - s.length) + s;
}

function genRow(at: Date): Row {
  const accepted = 8400 + Math.floor(Math.random() * 700);
  const processed = accepted - Math.floor(Math.random() * 40);
  const queue = 200 + Math.floor(Math.random() * 500);
  return {
    id: nextId++,
    ts: at.toISOString().slice(11, 19),
    accepted,
    processed,
    queue,
  };
}

const SEED_ROWS: Row[] = [
  { id: -3, ts: "08:42:21", accepted: 9013, processed: 8992, queue: 341 },
  { id: -2, ts: "08:42:16", accepted: 8714, processed: 8702, queue: 287 },
  { id: -1, ts: "08:42:11", accepted: 8421, processed: 8398, queue: 312 },
];

const AGE_OPACITY = [1, 0.55, 0.3];

function useTypewriter(
  text: string,
  speed: number,
  enabled: boolean,
): { shown: string; done: boolean } {
  const [n, setN] = useState(enabled ? 0 : text.length);
  useEffect(() => {
    if (!enabled) {
      setN(text.length);
      return;
    }
    setN(0);
    let raf = 0;
    const start = performance.now();
    const tick = (now: number) => {
      const elapsed = now - start;
      const next = Math.min(text.length, Math.floor(elapsed / speed));
      setN(next);
      if (next < text.length) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [text, speed, enabled]);
  return { shown: text.slice(0, n), done: n >= text.length };
}

function queueTone(q: number): string {
  if (q < 400) return "text-annotation";
  if (q < 600) return "text-amber-400";
  return "text-sev-p0";
}

function LogRow({
  row,
  index,
  reduced,
}: {
  row: Row;
  index: number;
  reduced: boolean;
}) {
  const isFresh = index === 0;
  const full =
    `${row.ts} ▎ accepted=${pad(row.accepted, 4)}/s ` +
    `processed=${pad(row.processed, 4)}/s queue=${pad(row.queue, 3)}`;
  const { shown, done } = useTypewriter(full, 22, isFresh && !reduced);

  return (
    <motion.div

      className={`relative flex items-center gap-2 pl-3 ${
        isFresh
          ? "border-l-2 border-accent shadow-[inset_2px_0_12px_-4px_var(--accent-glow)]"
          : "border-l-2 border-transparent"
      }`}
      style={{ opacity: AGE_OPACITY[index] ?? 0.3 }}
      initial={reduced ? false : { opacity: 0, y: -8 }}
      animate={{ opacity: AGE_OPACITY[index] ?? 0.3, y: 0 }}
      exit={reduced ? undefined : { opacity: 0, y: 12 }}
      transition={{ duration: 0.35, ease: [0.16, 1, 0.3, 1] }}
    >
      {isFresh && !done ? (

        <span className="font-mono text-meta text-text-secondary">
          {shown}
          <span
            className="ml-0.5 inline-block h-3 w-[7px] animate-pulse-live bg-accent align-middle"
            aria-hidden
          />
        </span>
      ) : (
        <ColoredRow row={row} fresh={isFresh} />
      )}
    </motion.div>
  );
}

function ColoredRow({ row, fresh }: { row: Row; fresh: boolean }) {
  const tsClass = fresh ? "text-text-primary" : "text-text-secondary";
  const baseClass = fresh ? "text-text-secondary" : "text-text-tertiary";
  return (
    <span className="font-mono text-meta">
      <span className={tsClass}>{row.ts}</span>
      <span className={`mx-1 ${baseClass}`}>▎</span>
      <span className={baseClass}>accepted=</span>
      <span className="text-accent tabular-nums">
        {pad(row.accepted, 4)}/s
      </span>
      <span className={`ml-2 ${baseClass}`}>processed=</span>
      <span
        className={`${fresh ? "text-text-primary" : "text-text-secondary"} tabular-nums`}
      >
        {pad(row.processed, 4)}/s
      </span>
      <span className={`ml-2 ${baseClass}`}>queue=</span>
      <span className={`${queueTone(row.queue)} tabular-nums`}>
        {pad(row.queue, 3)}
      </span>
    </span>
  );
}

export function LogTape() {
  const reduced = useReducedMotion();
  const [rows, setRows] = useState<Row[]>(SEED_ROWS);

  useEffect(() => {
    if (reduced) return;

    setRows([
      genRow(new Date(Date.now() - 10_000)),
      genRow(new Date(Date.now() - 5_000)),
      genRow(new Date()),
    ]);

    const id = setInterval(() => {
      setRows((current) => {
        const next = genRow(new Date());
        return [next, current[0], current[1]];
      });
    }, 5_000);
    return () => clearInterval(id);
  }, [reduced]);

  return (
    <div className="inline-block" suppressHydrationWarning>
      {}
      <div className="relative overflow-hidden rounded-md border border-border-subtle bg-bg-surface/95 shadow-[0_0_40px_-20px_var(--accent-glow)] backdrop-blur-[2px]">
        {}
        <div className="flex items-center justify-between border-b border-border-subtle bg-bg-base/60 px-3 py-1.5">
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1.5" aria-hidden>
              <span className="h-2 w-2 rounded-full bg-sev-p0/70" />
              <span className="h-2 w-2 rounded-full bg-sev-p2/70" />
              <span className="h-2 w-2 rounded-full bg-accent/70" />
            </span>
            <span className="font-mono text-[10px] uppercase tracking-[0.05em] text-text-tertiary">
              /var/log/vellum/metrics.log
            </span>
          </div>
          <span className="flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-[0.05em] text-text-secondary">
            <span
              className="inline-block h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
              aria-hidden
            />
            <span className="text-accent">live</span>
            <span className="text-text-tertiary">· 5s</span>
          </span>
        </div>

        {}
        <ScanlineTexture reduced={!!reduced} />

        {}
        <div className="relative px-4 py-3">
          <AnimatePresence initial={false}>
            {rows.map((r, i) => (
              <LogRow key={r.id} row={r} index={i} reduced={!!reduced} />
            ))}
          </AnimatePresence>
        </div>
      </div>
    </div>
  );
}

function ScanlineTexture({ reduced }: { reduced: boolean }) {
  return (
    <svg
      className="pointer-events-none absolute inset-0 h-full w-full"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <defs>
        <pattern
          id="scanlines"
          width="6"
          height="6"
          patternUnits="userSpaceOnUse"
          patternTransform="rotate(15)"
        >
          <line
            x1="0"
            y1="0"
            x2="0"
            y2="6"
            stroke="rgba(190,242,100,0.05)"
            strokeWidth="1"
          />
        </pattern>
      </defs>
      <motion.rect
        x={-12}
        y={-12}
        width="200%"
        height="200%"
        fill="url(#scanlines)"
        initial={{ x: -12, y: -12 }}
        animate={reduced ? undefined : { x: 0, y: 0 }}
        transition={
          reduced
            ? undefined
            : {
                duration: 10,
                repeat: Infinity,
                repeatType: "reverse",
                ease: "linear",
              }
        }
      />
    </svg>
  );
}
