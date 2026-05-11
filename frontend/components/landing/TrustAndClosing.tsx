"use client";

import { motion, useInView, useReducedMotion } from "framer-motion";
import Link from "next/link";
import { useRef } from "react";
import {
  siGo,
  siMongodb,
  siNextdotjs,
  siPostgresql,
  siRedis,
  siTimescale,
  type SimpleIcon,
} from "simple-icons";

interface Logo {
  icon: SimpleIcon;
  label: string;
}

const LOGOS: Logo[] = [
  { icon: siGo, label: "Go" },
  { icon: siPostgresql, label: "PostgreSQL" },
  { icon: siMongodb, label: "MongoDB" },
  { icon: siRedis, label: "Redis" },
  { icon: siTimescale, label: "TimescaleDB" },
  { icon: siNextdotjs, label: "Next.js" },
];

export function TrustAndClosing() {
  return (
    <section className="relative isolate overflow-hidden border-t border-divider px-6 pb-24 pt-24 sm:pb-32 sm:pt-32">
      {}
      <SectionGrid />

      <div className="relative mx-auto max-w-[1120px] space-y-24">
        {}
        <TrustStrip />

        {}
        <ClosingCard />
      </div>
    </section>
  );
}

function TrustStrip() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15%" });

  const reduced = !!useReducedMotion();

  return (
    <div ref={ref} className="text-center">
      {}
      <p className="inline-flex items-center justify-center gap-2 font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
        <span
          className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
          aria-hidden
        />
        Built with
      </p>
      <ul
        className="mt-8 flex flex-wrap items-center justify-center gap-10"
        aria-label="Tech stack"
      >
        {LOGOS.map(({ icon, label }, i) => (
          <motion.li
            key={label}
            className="group relative flex flex-col items-center gap-2"

            initial={reduced ? false : { opacity: 0, y: 8 }}
            animate={
              reduced
                ? undefined
                : inView
                  ? { opacity: 1, y: 0 }
                  : undefined
            }
            transition={{
              duration: 0.5,
              delay: 0.15 + i * 0.06,
              ease: [0.16, 1, 0.3, 1],
            }}
          >
            <span
              title={label}

              className="text-text-primary opacity-55 transition-[opacity,filter] duration-base ease-out group-hover:opacity-100 group-hover:[filter:drop-shadow(0_0_8px_rgba(190,242,100,0.55))]"
            >
              <svg
                role="img"
                aria-label={label}
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
                className="h-8 w-8 fill-current"
              >
                <title>{label}</title>
                <path d={icon.path} />
              </svg>
            </span>
            {}
            <span
              className="h-3 font-mono text-meta uppercase tracking-[0.05em] text-text-tertiary opacity-0 transition-opacity duration-base ease-out group-hover:opacity-100"
              aria-hidden
            >
              {label}
            </span>
          </motion.li>
        ))}
      </ul>
    </div>
  );
}

function ClosingCard() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15%" });

  const reduced = !!useReducedMotion();

  return (
    <div
      ref={ref}

      className="relative mx-auto max-w-[720px] border border-border-subtle bg-bg-surface px-8 py-20 text-center sm:py-24"
    >
      {}
      <div
        className="pointer-events-none absolute left-1/2 top-[40%] -z-10 h-32 w-[420px] -translate-x-1/2 -translate-y-1/2 rounded-[50%] shadow-[0_0_120px_30px_rgba(190,242,100,0.10)_inset]"
        aria-hidden
      />

      {}
      <CardCornerTicks inView={inView} reduced={reduced} />

      <h2 className="relative inline-block font-sans text-[32px] font-medium leading-[1.2] text-text-primary sm:text-[36px]">
        Stop reading errors.
        <br />
        Start resolving{" "}
        <span className="font-serif italic text-text-primary">incidents</span>.
        {}
        <ScanLine inView={inView} reduced={reduced} />
      </h2>

      <div className="mt-10 flex flex-col items-center gap-3">
        <Link
          href="/dashboard"

          className="inline-flex items-center gap-1.5 rounded-sm bg-accent px-6 py-3 font-sans text-[14px] font-medium text-accent-text transition-[background-color,box-shadow] duration-fast ease-out hover:bg-accent-bright hover:shadow-[0_0_28px_-6px_rgba(190,242,100,0.55)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-base"
        >
          Open the dashboard ›
        </Link>
        <Link
          href="https://github.com/kubeboiii/vellum/blob/main/docs/01-architecture.md"
          target="_blank"
          rel="noreferrer"
          className="font-sans text-body text-text-secondary transition-colors duration-fast hover:text-text-primary"
        >
          Or read the engineering writeup ↗
        </Link>
      </div>

    </div>
  );
}

function SectionGrid() {

  const reduced = !!useReducedMotion();
  return (
    <svg
      className="pointer-events-none absolute inset-0 h-full w-full"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <defs>
        <pattern
          id="closing-grid"
          width="56"
          height="56"
          patternUnits="userSpaceOnUse"
        >
          <path
            d="M 56 0 L 0 0 0 56"
            fill="none"
            stroke="rgba(255,255,255,0.02)"
            strokeWidth="1"
          />
        </pattern>
      </defs>
      <motion.rect
        x={-56}
        y={-56}
        width="200%"
        height="200%"
        fill="url(#closing-grid)"
        initial={{ x: -56, y: -56 }}
        animate={reduced ? undefined : { x: 0, y: 0 }}
        transition={
          reduced
            ? undefined
            : {
                duration: 24,
                repeat: Infinity,
                repeatType: "reverse",
                ease: "linear",
              }
        }
      />
    </svg>
  );
}

function CardCornerTicks({
  inView,
  reduced,
}: {
  inView: boolean;
  reduced: boolean;
}) {

  const corners: Array<{ pos: string; d: string }> = [
    { pos: "-top-px -left-px", d: "M 0 14 L 0 0 L 14 0" },
    { pos: "-top-px -right-px", d: "M 0 0 L 14 0 L 14 14" },
    { pos: "-bottom-px -left-px", d: "M 0 0 L 0 14 L 14 14" },
    { pos: "-bottom-px -right-px", d: "M 0 14 L 14 14 L 14 0" },
  ];
  return (
    <>
      {corners.map((c, i) => (
        <svg
          key={i}
          width="14"
          height="14"
          viewBox="0 0 14 14"
          fill="none"
          className={`pointer-events-none absolute ${c.pos}`}
          aria-hidden
        >
          <motion.path
            d={c.d}
            stroke="var(--accent)"
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
              delay: 0.2 + i * 0.1,
              ease: [0.16, 1, 0.3, 1],
            }}
          />
        </svg>
      ))}
    </>
  );
}

function ScanLine({
  inView,
  reduced,
}: {
  inView: boolean;
  reduced: boolean;
}) {
  return (
    <motion.span
      aria-hidden
      className="pointer-events-none absolute -bottom-3 left-0 block h-px bg-accent"
      initial={reduced ? false : { width: 0, opacity: 0 }}
      animate={
        reduced
          ? { width: "24px", opacity: 0.9 }
          : inView
            ? { width: ["0%", "100%", "24px"], opacity: [0, 0.9, 0.9] }
            : undefined
      }
      transition={{
        duration: 1.6,
        delay: 0.6,
        times: [0, 0.6, 1],
        ease: [0.16, 1, 0.3, 1],
      }}
    />
  );
}
