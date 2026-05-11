// LANDING.md §5.9 — trust strip + closing card.
//
// Phase-5 polish promotes the closing from a plain centered card
// into the page's "deliberate moment":
//
//   * BUILT WITH chip with a pulsing lime dot prefix
//   * Each logo hovers a lime halo + the tech name fades in beneath
//   * Drifting grid texture sits behind the closing card
//   * Closing card replaces its rounded-lg border with four animated
//     lime corner ticks (technical-drawing crop marks)
//   * The headline gets one italic-serif word ("calmly") — mirrors
//     the Hero's serif moment so the page closes on the same chord
//   * A scanning lime line crosses the headline on first reveal
//   * Primary CTA gets the same lime halo as the rest of the app
//
// The hand-drawn annotation arrow we tried here was removed: at the
// closing card's actual rendered size the arrow sat awkwardly over
// the CTA rather than pointing at it. Decisions.md 2026-05-11.
//
// Per THEME.md §8.4: no gradients. Box-shadows only.

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

// Order matches the stack table in CLAUDE.md.
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
      {/* Ambient drifting grid across the whole section. */}
      <SectionGrid />

      <div className="relative mx-auto max-w-[1120px] space-y-24">
        {/* ---- Trust strip ---- */}
        <TrustStrip />

        {/* ---- Closing card ---- */}
        <ClosingCard />
      </div>
    </section>
  );
}

// ──────────────────────────────────────────────────────────────────
// Trust strip
// ──────────────────────────────────────────────────────────────────

function TrustStrip() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15%" });
  // useReducedMotion returns boolean | null. Coerce to strict bool.
  const reduced = !!useReducedMotion();

  return (
    <div ref={ref} className="text-center">
      {/* BUILT WITH chip — pulsing lime dot prefix matches the
          landing's "live signal" vocabulary. */}
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
            // Fade each logo in with a 60ms cascade so they look
            // like they "land" left-to-right rather than appearing
            // as a block.
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
              // Mono single-color: text-text-primary at low opacity.
              // Hover bumps to full opacity AND emits a lime halo.
              // The halo is shadowed on the SVG itself via filter.
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
            {/* Tech name fades in below on hover. Reserved-height
                slot keeps the row layout stable. */}
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

// ──────────────────────────────────────────────────────────────────
// Closing card
// ──────────────────────────────────────────────────────────────────

function ClosingCard() {
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, margin: "-15%" });
  // useReducedMotion returns boolean | null. Coerce to strict bool.
  const reduced = !!useReducedMotion();

  return (
    <div
      ref={ref}
      // No rounded-lg here on purpose — the technical-drawing corner
      // ticks DO the framing. A subtle border-subtle still anchors
      // the surface against the section's grid texture.
      className="relative mx-auto max-w-[720px] border border-border-subtle bg-bg-surface px-8 py-20 text-center sm:py-24"
    >
      {/* Soft inset halo behind the headline — no gradient, just a
          large faint lime box-shadow on a transparent anchor. */}
      <div
        className="pointer-events-none absolute left-1/2 top-[40%] -z-10 h-32 w-[420px] -translate-x-1/2 -translate-y-1/2 rounded-[50%] shadow-[0_0_120px_30px_rgba(190,242,100,0.10)_inset]"
        aria-hidden
      />

      {/* Animated corner ticks — replace the would-be rounded
          corners with 4 lime L-marks that draw themselves on
          reveal. Crop-mark vocabulary. */}
      <CardCornerTicks inView={inView} reduced={reduced} />

      <h2 className="relative inline-block font-sans text-[32px] font-medium leading-[1.2] text-text-primary sm:text-[36px]">
        Stop reading errors.
        <br />
        Start resolving{" "}
        <span className="font-serif italic text-text-primary">incidents</span>.
        {/* Scanning lime line that sweeps the headline once on
            reveal — mirrors the dashboard's section-header rule
            but as a single pass instead of a static accent. */}
        <ScanLine inView={inView} reduced={reduced} />
      </h2>

      <div className="mt-10 flex flex-col items-center gap-3">
        <Link
          href="/dashboard"
          // Lime halo on hover — same vocabulary as the Hero's
          // primary CTA and the RCA-form Submit button.
          className="inline-flex items-center gap-1.5 rounded-sm bg-accent px-6 py-3 font-sans text-[14px] font-medium text-accent-text transition-[background-color,box-shadow] duration-fast ease-out hover:bg-accent-bright hover:shadow-[0_0_28px_-6px_rgba(190,242,100,0.55)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-base"
        >
          Open the dashboard ›
        </Link>
        <Link
          href="https://github.com/kubeboiii/ims/blob/main/docs/01-architecture.md"
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

// ──────────────────────────────────────────────────────────────────
// Decorative sub-components
// ──────────────────────────────────────────────────────────────────

// SectionGrid: ambient drifting grid behind the whole section.
function SectionGrid() {
  // useReducedMotion returns boolean | null. Coerce to strict bool.
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

// CardCornerTicks: 4 lime L-marks at each corner of the closing
// card, drawing themselves in via stroke-dashoffset on reveal.
function CardCornerTicks({
  inView,
  reduced,
}: {
  inView: boolean;
  reduced: boolean;
}) {
  // Each corner: position + path. The path is 14px on each leg.
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

// ScanLine: a lime hairline that sweeps left-to-right under the
// headline on first reveal, then settles into a short accent.
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
