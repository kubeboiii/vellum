"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import { useRef } from "react";

export function ProblemStrip() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15%" });

  const reduced = !!useReducedMotion();

  const runs: Run[] = [
    { text: "Production breaks.", emphasis: true },
    {
      text:
        " Errors arrive by the thousand from APIs, caches, queues, databases.",
    },
    {
      text: " You can't read them. You can't sort them. You can't even keep up.",
      emphasis: true,
    },
    {
      text:
        " Vellum turns that flood into a small, structured list of incidents your team can actually work.",
      annotation: true,
    },
  ];

  return (
    <section
      ref={ref}
      className="relative isolate overflow-hidden border-t border-divider px-6 py-24 sm:py-32"
    >
      {}
      <GridTexture />

      {}
      <CornerTicks inView={inView} reduced={reduced} />

      <div className="relative mx-auto max-w-[720px]">
        {}
        <div
          className="pointer-events-none absolute inset-x-[-10%] inset-y-[-30%] -z-10"
          aria-hidden
        >
          <div className="mx-auto h-full w-full max-w-[640px] rounded-[50%] opacity-60 shadow-[0_0_120px_60px_rgba(190,242,100,0.06)_inset]" />
        </div>

        <p className="font-sans text-[22px] leading-[1.45] text-text-secondary sm:text-[24px]">
          {runs.map((run, i) => (
            <RunSpan
              key={i}
              run={run}
              inView={inView}
              reduced={reduced}
              priorWords={runs.slice(0, i).reduce((a, r) => a + wordCount(r.text), 0)}
            />
          ))}
        </p>
      </div>
    </section>
  );
}

interface Run {
  text: string;
  emphasis?: boolean;
  annotation?: boolean;
}

function wordCount(s: string): number {
  return s.trim().split(/\s+/).length;
}

function RunSpan({
  run,
  inView,
  reduced,
  priorWords,
}: {
  run: Run;
  inView: boolean;
  reduced: boolean;
  priorWords: number;
}) {

  const leading = /^\s+/.exec(run.text)?.[0] ?? "";
  const words = run.text.trim().split(/\s+/);

  const cls = run.emphasis
    ? "relative font-medium text-text-primary"
    : "";

  return (
    <span className={cls}>
      {leading}
      {words.map((w, j) => (
        <motion.span
          key={j}
          className="inline-block"

          initial={reduced ? false : { opacity: 0, y: 6 }}
          animate={
            reduced
              ? undefined
              : inView
                ? { opacity: 1, y: 0 }
                : undefined
          }
          transition={{
            duration: 0.45,
            delay: 0.02 * (priorWords + j),
            ease: [0.16, 1, 0.3, 1],
          }}
        >
          {w}
          {j < words.length - 1 ? " " : ""}
        </motion.span>
      ))}
      {}
      {run.emphasis && (
        <EmphasisUnderline inView={inView} reduced={reduced} priorWords={priorWords} />
      )}
    </span>
  );
}

function EmphasisUnderline({
  inView,
  reduced,
  priorWords,
}: {
  inView: boolean;
  reduced: boolean;
  priorWords: number;
}) {
  return (
    <motion.span
      aria-hidden
      className="pointer-events-none absolute -bottom-0.5 left-0 right-0 block h-px bg-accent"
      initial={reduced ? false : { scaleX: 0, opacity: 0 }}
      style={{ transformOrigin: "left center" }}
      animate={
        reduced
          ? undefined
          : inView
            ? { scaleX: 1, opacity: 0.7 }
            : undefined
      }
      transition={{
        duration: 0.7,

        delay: 0.02 * (priorWords + 8),
        ease: [0.16, 1, 0.3, 1],
      }}
    />
  );
}

function CornerTicks({ inView, reduced }: { inView: boolean; reduced: boolean }) {
  const corners: Array<{ x: string; y: string; d: string }> = [
    { x: "24", y: "24", d: "M 0 12 L 0 0 L 12 0" },
    { x: "calc(100% - 36px)", y: "24", d: "M 0 0 L 12 0 L 12 12" },
    { x: "24", y: "calc(100% - 36px)", d: "M 0 0 L 0 12 L 12 12" },
    {
      x: "calc(100% - 36px)",
      y: "calc(100% - 36px)",
      d: "M 0 12 L 12 12 L 12 0",
    },
  ];
  return (
    <>
      {corners.map((c, i) => (
        <svg
          key={i}
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          className="pointer-events-none absolute"
          style={{ left: c.x, top: c.y }}
          aria-hidden
        >
          <motion.path
            d={c.d}
            stroke="var(--accent)"
            strokeOpacity="0.55"
            strokeWidth="1"
            strokeLinecap="square"
            initial={reduced ? false : { pathLength: 0, opacity: 0 }}
            animate={
              reduced
                ? undefined
                : inView
                  ? { pathLength: 1, opacity: 0.7 }
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

function GridTexture() {
  const reduced = useReducedMotion();
  return (
    <svg
      className="pointer-events-none absolute inset-0 h-full w-full opacity-70"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <defs>
        <pattern
          id="problem-grid"
          width="48"
          height="48"
          patternUnits="userSpaceOnUse"
        >
          <path
            d="M 48 0 L 0 0 0 48"
            fill="none"
            stroke="rgba(255,255,255,0.02)"
            strokeWidth="1"
          />
        </pattern>
      </defs>
      <motion.rect
        x={-48}
        y={-48}
        width="200%"
        height="200%"
        fill="url(#problem-grid)"
        initial={{ x: -48, y: -48 }}
        animate={reduced ? undefined : { x: 0, y: 0 }}
        transition={
          reduced
            ? undefined
            : {
                duration: 22,
                repeat: Infinity,
                repeatType: "reverse",
                ease: "linear",
              }
        }
      />
    </svg>
  );
}
