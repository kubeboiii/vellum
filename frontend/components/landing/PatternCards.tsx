// LANDING.md §5.6 — architecture pattern cards.
//
// Three cards. Each has:
//   * Its own accent color (red / violet / lime) keyed to the
//     architectural metaphor of the diagram inside it
//   * A unique on-canvas animation (capacity ticks pulse, lua box
//     glows, lime arrow draws itself on hover)
//   * A subtle glow on hover, color-matched to the accent. NOT a
//     gradient (THEME.md §8.4) — just a box-shadow that fades in.
//   * Stage label + a small "tech tag" above the title so the cards
//     read like platform-engineering doc-site cards.
//
// All motion is gated on framer-motion's `useReducedMotion`.

"use client";

import { motion, useReducedMotion } from "framer-motion";
import Link from "next/link";

// Per-card accent. Drives the title/glow/diagram color so each card
// is unmistakably distinct at a glance.
type Accent = "red" | "violet" | "lime";

const ACCENT_CLASSES: Record<
  Accent,
  { glow: string; ring: string; tag: string; readMore: string }
> = {
  red: {
    glow: "hover:shadow-[0_0_32px_-12px_rgba(239,68,68,0.45)]",
    ring: "hover:border-sev-p0-border",
    tag: "border-sev-p0-border bg-sev-p0-bg text-red-300",
    readMore: "text-red-300 hover:text-red-200",
  },
  violet: {
    glow: "hover:shadow-[0_0_32px_-12px_rgba(167,139,250,0.45)]",
    ring: "hover:border-annotation-dim",
    tag: "border-annotation-dim bg-annotation/10 text-annotation",
    readMore: "text-annotation hover:text-annotation",
  },
  lime: {
    glow: "hover:shadow-[0_0_32px_-12px_var(--accent-glow)]",
    ring: "hover:border-accent-border",
    tag: "border-accent-border bg-accent-bg text-accent",
    readMore: "text-accent hover:text-accent-bright",
  },
};

interface PatternCardProps {
  tag: string;
  title: string;
  description: string;
  diagram: React.ReactNode;
  href: string;
  accent: Accent;
  index: number; // for stagger
}

function PatternCard({
  tag,
  title,
  description,
  diagram,
  href,
  accent,
  index,
}: PatternCardProps) {
  const a = ACCENT_CLASSES[accent];
  return (
    <motion.article
      // Each card slides in from below with a slight stagger so the
      // row reads left-to-right rather than landing all at once.
      initial={{ opacity: 0, y: 18 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-10%" }}
      transition={{ duration: 0.5, delay: index * 0.08, ease: [0.16, 1, 0.3, 1] }}
      className={`group flex flex-col overflow-hidden rounded-lg border border-border-subtle bg-bg-surface transition-all duration-base ease-out ${a.ring} ${a.glow}`}
    >
      <div className="relative border-b border-border-subtle bg-bg-base p-8">
        {/* Accent tag in the top-left of the diagram area. */}
        <span
          className={`absolute left-4 top-4 inline-flex items-center gap-1.5 rounded-sm border px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-[0.05em] ${a.tag}`}
        >
          {tag}
        </span>
        {diagram}
      </div>
      <div className="flex flex-1 flex-col p-6">
        <h3 className="font-sans text-[18px] font-medium text-text-primary">
          {title}
        </h3>
        <p className="mt-2 flex-1 font-sans text-body leading-[1.55] text-text-secondary">
          {description}
        </p>
        <Link
          href={href}
          target="_blank"
          rel="noreferrer"
          className={`mt-6 font-mono text-meta uppercase tracking-[0.04em] transition-colors duration-fast ${a.readMore}`}
        >
          Read more ↗
        </Link>
      </div>
    </motion.article>
  );
}

export function PatternCards() {
  return (
    <section className="border-t border-divider px-6 py-24 sm:py-32">
      <div className="mx-auto max-w-[1120px]">
        <header className="mb-12">
          <p className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            Architecture patterns
          </p>
          <h2 className="mt-3 font-sans text-[28px] font-medium leading-[1.2] text-text-primary sm:text-[36px]">
            Three decisions worth defending.
          </h2>
        </header>

        <div className="grid gap-4 lg:grid-cols-3">
          <PatternCard
            index={0}
            accent="red"
            tag="Backpressure"
            title="Backpressured ingestion"
            description="Bounded channel + worker pool. When full, return 503 — never block, never crash. The entire backpressure story is one non-blocking select."
            diagram={<DiagramIngestion />}
            href="https://github.com/kubeboiii/ims/blob/main/docs/01-architecture.md#4-ingestion-pipeline--end-to-end-walkthrough"
          />
          <PatternCard
            index={1}
            accent="violet"
            tag="Atomicity"
            title="Atomic debouncing"
            description="One Redis Lua script collapses up to 100 correlated signals into one work item. Server-side single-threaded execution means no race, even under multi-replica deploys."
            diagram={<DiagramDebounce />}
            href="https://github.com/kubeboiii/ims/blob/main/docs/01-architecture.md#5-debounce-engine"
          />
          <PatternCard
            index={2}
            accent="lime"
            tag="State machine"
            title="Stateful workflow"
            description="State pattern enforces no CLOSE without RCA. SERIALIZABLE Postgres transactions wrap each transition, and MTTR is computed automatically on close."
            diagram={<DiagramWorkflow />}
            href="https://github.com/kubeboiii/ims/blob/main/docs/01-architecture.md#7-design-patterns-in-this-system"
          />
        </div>
      </div>
    </section>
  );
}

// =============================================================
// DIAGRAM 1 — Ingestion. Pulsing capacity ticks animate to suggest
// the channel filling and draining. The motion language is "fluid
// flow" — bars pulse in a wave.
// =============================================================

function DiagramIngestion() {
  const reduced = useReducedMotion();
  // 8 capacity ticks spread across the channel width.
  const tickXs = [100, 112, 124, 136, 148, 160, 172, 184];
  return (
    <svg viewBox="0 0 280 200" xmlns="http://www.w3.org/2000/svg" className="w-full" aria-hidden>
      <defs>
        <marker
          id="arr-ing"
          viewBox="0 0 10 10"
          refX="8"
          refY="5"
          markerWidth="6"
          markerHeight="6"
          orient="auto-start-reverse"
        >
          <path d="M0,0 L10,5 L0,10 z" fill="var(--diagram-stroke)" />
        </marker>
      </defs>

      <rect
        x="100"
        y="14"
        width="80"
        height="28"
        rx="6"
        fill="transparent"
        stroke="var(--diagram-stroke)"
      />
      <text
        x="140"
        y="32"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        HTTP
      </text>

      <path
        d="M 140 42 L 140 70"
        stroke="var(--diagram-stroke)"
        fill="none"
        markerEnd="url(#arr-ing)"
      />

      {/* CHANNEL box — red outline. */}
      <rect
        x="60"
        y="70"
        width="160"
        height="50"
        rx="6"
        fill="transparent"
        stroke="var(--sev-p0)"
      />
      <text
        x="140"
        y="92"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        CHANNEL · 50K
      </text>

      {/* Animated capacity ticks. Each pulses in a left-to-right
          wave to suggest signals streaming through the queue. */}
      {tickXs.map((x, i) => (
        <motion.line
          key={x}
          x1={x}
          y1="106"
          x2={x}
          y2="112"
          stroke="var(--sev-p0)"
          strokeWidth="1.5"
          strokeLinecap="round"
          initial={{ opacity: 0.3 }}
          animate={reduced ? undefined : { opacity: [0.3, 1, 0.3] }}
          transition={
            reduced
              ? undefined
              : {
                  duration: 1.6,
                  delay: i * 0.12,
                  repeat: Infinity,
                  ease: "easeInOut",
                }
          }
        />
      ))}

      <path
        d="M 140 120 L 140 148"
        stroke="var(--diagram-stroke)"
        fill="none"
        markerEnd="url(#arr-ing)"
      />
      <rect
        x="60"
        y="148"
        width="160"
        height="32"
        rx="6"
        fill="transparent"
        stroke="var(--diagram-stroke)"
      />
      <text
        x="140"
        y="168"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        WORKERS × 16
      </text>
    </svg>
  );
}

// =============================================================
// DIAGRAM 2 — Debounce. The Lua box has a slow violet glow that
// pulses (CSS box-shadow on the rect via filter), and a violet dot
// travels SIGNAL → LUA → WORK ITEM continuously to suggest data
// streaming through the atomic check.
// =============================================================

function DiagramDebounce() {
  const reduced = useReducedMotion();
  return (
    <svg viewBox="0 0 280 200" xmlns="http://www.w3.org/2000/svg" className="w-full" aria-hidden>
      <defs>
        <marker
          id="arr-deb"
          viewBox="0 0 10 10"
          refX="8"
          refY="5"
          markerWidth="6"
          markerHeight="6"
          orient="auto-start-reverse"
        >
          <path d="M0,0 L10,5 L0,10 z" fill="var(--annotation)" />
        </marker>
        {/* SVG filter for the violet glow on the Lua box. Cheaper
            than animating box-shadow on a foreignObject; runs on the
            GPU. */}
        <filter id="lua-glow" x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="3" result="b" />
          <feMerge>
            <feMergeNode in="b" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      <rect
        x="80"
        y="18"
        width="120"
        height="32"
        rx="6"
        fill="transparent"
        stroke="var(--diagram-stroke)"
      />
      <text
        x="140"
        y="38"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        SIGNAL
      </text>

      <path
        d="M 140 50 L 140 80"
        stroke="var(--annotation)"
        fill="none"
        markerEnd="url(#arr-deb)"
      />

      {/* Lua box — pulses opacity (essentially a soft glow rhythm). */}
      <motion.rect
        x="60"
        y="80"
        width="160"
        height="50"
        rx="6"
        fill="transparent"
        stroke="var(--annotation)"
        strokeWidth="1.5"
        filter="url(#lua-glow)"
        animate={reduced ? undefined : { opacity: [0.6, 1, 0.6] }}
        transition={reduced ? undefined : { duration: 2.2, repeat: Infinity, ease: "easeInOut" }}
      />
      <text
        x="140"
        y="100"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        REDIS LUA
      </text>
      <text
        x="140"
        y="118"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="10"
        fill="var(--annotation)"
        fontStyle="italic"
      >
        atomic check
      </text>

      <path
        d="M 140 130 L 140 160"
        stroke="var(--annotation)"
        fill="none"
        markerEnd="url(#arr-deb)"
      />
      <rect
        x="80"
        y="160"
        width="120"
        height="32"
        rx="6"
        fill="transparent"
        stroke="var(--diagram-stroke)"
      />
      <text
        x="140"
        y="180"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        WORK ITEM
      </text>

      {/* Traveling violet dot along the SIGNAL→LUA→WORK ITEM column. */}
      {!reduced && (
        <motion.circle
          r="2.5"
          fill="var(--annotation)"
          cx="140"
          animate={{ cy: [50, 80, 130, 160] }}
          transition={{
            duration: 2.4,
            repeat: Infinity,
            ease: "easeInOut",
            times: [0, 0.33, 0.66, 1],
          }}
        />
      )}
    </svg>
  );
}

// =============================================================
// DIAGRAM 3 — Workflow. The lime RESOLVED→CLOSED arrow draws
// itself on first reveal (stroke-dashoffset animation), then the
// CLOSED box pulses its lime border once to signal "the gate fired".
// The earlier OPEN→INVESTIGATING→RESOLVED edges remain static.
// =============================================================

function DiagramWorkflow() {
  const reduced = useReducedMotion();
  return (
    <svg viewBox="0 0 280 200" xmlns="http://www.w3.org/2000/svg" className="w-full" aria-hidden>
      <defs>
        <marker
          id="arr-wf"
          viewBox="0 0 10 10"
          refX="8"
          refY="5"
          markerWidth="6"
          markerHeight="6"
          orient="auto-start-reverse"
        >
          <path d="M0,0 L10,5 L0,10 z" fill="var(--diagram-stroke)" />
        </marker>
        <marker
          id="arr-wf-active"
          viewBox="0 0 10 10"
          refX="8"
          refY="5"
          markerWidth="6"
          markerHeight="6"
          orient="auto-start-reverse"
        >
          <path d="M0,0 L10,5 L0,10 z" fill="var(--accent)" />
        </marker>
      </defs>

      <rect x="12" y="20" width="100" height="32" rx="6" fill="transparent" stroke="var(--diagram-stroke)" />
      <text x="62" y="40" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="11" fill="var(--text-primary)" letterSpacing="0.04em">OPEN</text>

      <path d="M 112 36 L 152 36" stroke="var(--diagram-stroke)" fill="none" markerEnd="url(#arr-wf)" />
      <rect x="152" y="20" width="116" height="32" rx="6" fill="transparent" stroke="var(--diagram-stroke)" />
      <text x="210" y="40" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="11" fill="var(--text-primary)" letterSpacing="0.04em">INVESTIGATING</text>

      <path d="M 210 52 L 210 88" stroke="var(--diagram-stroke)" fill="none" markerEnd="url(#arr-wf)" />

      <rect x="152" y="88" width="116" height="32" rx="6" fill="transparent" stroke="var(--diagram-stroke)" />
      <text x="210" y="108" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="11" fill="var(--text-primary)" letterSpacing="0.04em">RESOLVED</text>

      {/* Lime gated edge — animates "drawing itself" on first
          render via stroke-dashoffset. The dasharray covers the
          path length; we animate offset from full length to 0. */}
      <motion.path
        d="M 152 104 L 112 104"
        stroke="var(--accent)"
        strokeWidth="1.5"
        fill="none"
        markerEnd="url(#arr-wf-active)"
        strokeDasharray="40"
        initial={reduced ? { strokeDashoffset: 0 } : { strokeDashoffset: 40 }}
        whileInView={{ strokeDashoffset: 0 }}
        viewport={{ once: true, margin: "-20%" }}
        transition={{ duration: 0.8, ease: "easeOut", delay: 0.3 }}
      />

      {/* CLOSED box — pulses its lime border once when the section
          enters view, signaling "the rule fired". */}
      <motion.rect
        x="12"
        y="88"
        width="100"
        height="32"
        rx="6"
        fill="transparent"
        stroke="var(--accent)"
        strokeWidth="1"
        initial={reduced ? undefined : { strokeWidth: 1 }}
        whileInView={reduced ? undefined : { strokeWidth: [1, 2, 1] }}
        viewport={{ once: true, margin: "-20%" }}
        transition={{ duration: 0.9, ease: "easeOut", delay: 1.1, repeat: 1, repeatType: "reverse" }}
      />
      <text x="62" y="108" textAnchor="middle" fontFamily="var(--font-mono)" fontSize="11" fill="var(--text-primary)" letterSpacing="0.04em">CLOSED</text>

      {/* Gate label — sits clearly BELOW the CLOSED/RESOLVED row
          so it doesn't overlap the lime arrow at y=104. A short
          dashed tick connects the arrow to the label so the eye
          knows the label is annotating that edge specifically. */}
      <line
        x1="132"
        y1="120"
        x2="132"
        y2="140"
        stroke="var(--accent)"
        strokeWidth="0.75"
        strokeDasharray="2 2"
      />
      <text
        x="132"
        y="152"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="10"
        fill="var(--accent)"
        letterSpacing="0.04em"
      >
        requires RCA
      </text>
    </svg>
  );
}
