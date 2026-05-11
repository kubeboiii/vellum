"use client";

import { IconAlertTriangle, IconCheck, IconCircleCheck, IconX } from "@tabler/icons-react";
import { motion, useReducedMotion } from "framer-motion";
import { useEffect, useState } from "react";

const LEFT_OUTCOMES = [
  "10,000+ signals/sec",
  "No correlation",
  "No state, no audit",
];

const RIGHT_OUTCOMES = [
  "100:1 noise reduction",
  "Single source of truth",
  "Mandatory RCA + auto MTTR",
];

export function Comparison() {
  return (
    <section id="product" className="border-t border-divider px-6 py-24 sm:py-32">
      <div className="mx-auto grid max-w-[1120px] gap-6 lg:grid-cols-2">
        {}
        <motion.div
          initial={{ opacity: 0, x: -20 }}
          whileInView={{ opacity: 1, x: 0 }}
          viewport={{ once: true, margin: "-10%" }}
          transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
        >
          <Card
            label="Without Vellum"
            dotClass="bg-sev-p0"
            glowClass="hover:shadow-[0_0_36px_-14px_rgba(239,68,68,0.5)] hover:border-sev-p0-border"
            statusChip={<StatusChipChaos />}
            diagram={<DiagramBefore />}
            statBar={<StatBarChaos />}
            outcomes={LEFT_OUTCOMES}
            OutcomeIcon={() => <IconX size={14} className="text-sev-p0" aria-hidden />}
            footnote="The on-call SRE is the rate limiter."
          />
        </motion.div>

        {}
        <motion.div
          initial={{ opacity: 0, x: 20 }}
          whileInView={{ opacity: 1, x: 0 }}
          viewport={{ once: true, margin: "-10%" }}
          transition={{ duration: 0.5, delay: 0.1, ease: [0.16, 1, 0.3, 1] }}
        >
          <Card
            label="With Vellum"
            dotClass="bg-accent"
            glowClass="hover:shadow-[0_0_36px_-14px_var(--accent-glow)] hover:border-accent-border"
            statusChip={<StatusChipStable />}
            diagram={<DiagramAfter />}
            statBar={<StatBarStable />}
            outcomes={RIGHT_OUTCOMES}
            OutcomeIcon={() => <IconCheck size={14} className="text-accent" aria-hidden />}
            footnote="One pipeline. One source of truth."
          />
        </motion.div>
      </div>
    </section>
  );
}

interface CardProps {
  label: string;
  dotClass: string;
  glowClass: string;
  statusChip: React.ReactNode;
  diagram: React.ReactNode;
  statBar: React.ReactNode;
  outcomes: string[];
  OutcomeIcon: () => React.ReactNode;
  footnote: string;
}

function Card({
  label,
  dotClass,
  glowClass,
  statusChip,
  diagram,
  statBar,
  outcomes,
  OutcomeIcon,
  footnote,
}: CardProps) {
  return (
    <article
      className={`flex min-h-[560px] flex-col rounded-lg border border-border-subtle bg-bg-surface p-8 transition-all duration-base ease-out ${glowClass}`}
    >
      {}
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <span className={`h-1.5 w-1.5 rounded-full ${dotClass}`} aria-hidden />
          <span className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            {label}
          </span>
        </div>
        {statusChip}
      </header>

      {}
      <div className="my-6 flex-1">{diagram}</div>

      {}
      <div className="border-t border-border-subtle pt-4">{statBar}</div>

      {}
      <ul className="mt-5 space-y-2.5 border-t border-border-subtle pt-5">
        {outcomes.map((o) => (
          <li key={o} className="flex items-center gap-3">
            <OutcomeIcon />
            <span className="font-mono text-data text-text-primary">{o}</span>
          </li>
        ))}
      </ul>

      {}
      <p className="mt-4 font-serif text-meta italic text-text-tertiary">
        {footnote}
      </p>
    </article>
  );
}

function StatusChipChaos() {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-sm border border-sev-p0-border bg-sev-p0-bg px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.05em] text-red-300">
      <IconAlertTriangle size={11} className="text-sev-p0" aria-hidden />
      <span className="animate-pulse-live">Incident · unhandled</span>
    </span>
  );
}

function StatusChipStable() {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-sm border border-accent-border bg-accent-bg px-2 py-0.5 font-mono text-[10px] uppercase tracking-[0.05em] text-accent">
      <IconCircleCheck size={11} aria-hidden />
      Active · 2 incidents
    </span>
  );
}

function JitterStat({
  label,
  base,
  jitter,
  unit,
  prefix = "",
}: {
  label: string;
  base: number;
  jitter: number;
  unit: string;
  prefix?: string;
}) {
  const reduced = useReducedMotion();
  const [val, setVal] = useState(base);
  useEffect(() => {
    if (reduced) return;
    const id = setInterval(() => {
      setVal(base + Math.floor((Math.random() - 0.5) * jitter * 2));
    }, 600);
    return () => clearInterval(id);
  }, [base, jitter, reduced]);
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-mono text-[9px] uppercase tracking-[0.05em] text-text-tertiary">
        {label}
      </span>
      <span className="font-mono text-data tabular-nums text-text-primary">
        {prefix}
        {val.toLocaleString()}
        <span className="ml-0.5 text-text-tertiary">{unit}</span>
      </span>
    </div>
  );
}

function StaticStat({
  label,
  value,
  tone = "primary",
}: {
  label: string;
  value: string;
  tone?: "primary" | "accent" | "muted";
}) {
  const toneClass =
    tone === "accent"
      ? "text-accent"
      : tone === "muted"
        ? "text-text-tertiary"
        : "text-text-primary";
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-mono text-[9px] uppercase tracking-[0.05em] text-text-tertiary">
        {label}
      </span>
      <span className={`font-mono text-data tabular-nums ${toneClass}`}>{value}</span>
    </div>
  );
}

function StatBarChaos() {
  return (
    <div className="grid grid-cols-4 gap-3">
      <JitterStat label="Inbox" base={8712} jitter={140} unit="" />
      <StaticStat label="Correlation" value="—" tone="muted" />
      <StaticStat label="MTTR" value="?" tone="muted" />
      <StaticStat label="Audit" value="none" tone="muted" />
    </div>
  );
}

function StatBarStable() {
  return (
    <div className="grid grid-cols-4 gap-3">
      <StaticStat label="Queue" value="312 / 50K" />
      <StaticStat label="p99" value="1.89 ms" tone="accent" />
      <StaticStat label="MTTR" value="12 m" />
      <StaticStat label="RCA" value="100%" tone="accent" />
    </div>
  );
}

function DiagramBefore() {
  const reduced = useReducedMotion();
  const sources = [
    { label: "API ERRORS", base: 2300, jitter: 200, period: 1.4, delay: 0.0 },
    { label: "CACHE FAILS", base: 1100, jitter: 120, period: 1.1, delay: 0.4 },
    { label: "DB TIMEOUTS", base: 480, jitter: 80, period: 1.6, delay: 0.2 },
    { label: "Q BACKLOG", base: 720, jitter: 90, period: 1.3, delay: 0.7 },
    { label: "K8S OOM", base: 220, jitter: 40, period: 1.7, delay: 0.5 },
  ];

  return (
    <svg
      viewBox="0 0 360 260"
      xmlns="http://www.w3.org/2000/svg"
      className="h-full w-full"
      aria-hidden
    >
      <defs>
        <marker
          id="arr-p0"
          viewBox="0 0 10 10"
          refX="8"
          refY="5"
          markerWidth="6"
          markerHeight="6"
          orient="auto-start-reverse"
        >
          <path d="M0,0 L10,5 L0,10 z" fill="var(--sev-p0)" />
        </marker>
      </defs>

      {}
      <text
        x="356"
        y="14"
        textAnchor="end"
        fontFamily="var(--font-mono)"
        fontSize="9"
        fill="var(--text-tertiary)"
        letterSpacing="0.05em"
      >
        TOTAL · ~10K/s
      </text>

      {}
      <motion.rect
        x="12"
        y="110"
        width="88"
        height="44"
        rx="6"
        fill="transparent"
        stroke="var(--sev-p0)"
        strokeWidth="1"
        initial={{ strokeOpacity: 0.5 }}
        animate={reduced ? undefined : { strokeOpacity: [0.5, 1, 0.5] }}
        transition={reduced ? undefined : { duration: 1.4, repeat: Infinity, ease: "easeInOut" }}
      />
      <text
        x="56"
        y="132"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="11"
        fill="var(--text-primary)"
        letterSpacing="0.04em"
      >
        SRE
      </text>
      <text
        x="56"
        y="146"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="9"
        fill="var(--sev-p0)"
        letterSpacing="0.05em"
      >
        on-call · 1
      </text>

      {sources.map((s, i) => {
        const y = 30 + i * 44;
        return (
          <g key={s.label}>
            {}
            <text
              x="356"
              y={y + 4}
              textAnchor="end"
              fontFamily="var(--font-mono)"
              fontSize="10"
              fill="var(--diagram-label)"
              letterSpacing="0.04em"
            >
              {s.label}
            </text>
            {}
            <JitterRate
              x={356}
              y={y + 16}
              base={s.base}
              jitter={s.jitter}
              delay={s.delay}
            />
            {}
            <motion.path
              d={`M 230 ${y} L 104 ${y < 132 ? y + 14 : y - 6}`}
              stroke="var(--sev-p0)"
              strokeWidth="1"
              strokeDasharray="4 4"
              fill="none"
              markerEnd="url(#arr-p0)"
              initial={{ opacity: reduced ? 0.7 : 0.25 }}
              animate={reduced ? undefined : { opacity: [0.25, 1, 0.25] }}
              transition={
                reduced
                  ? undefined
                  : {
                      duration: s.period,
                      delay: s.delay,
                      repeat: Infinity,
                      ease: "easeInOut",
                    }
              }
            />
          </g>
        );
      })}
    </svg>
  );
}

function JitterRate({
  x,
  y,
  base,
  jitter,
  delay,
}: {
  x: number;
  y: number;
  base: number;
  jitter: number;
  delay: number;
}) {
  const reduced = useReducedMotion();
  const [val, setVal] = useState(base);
  useEffect(() => {
    if (reduced) return;
    const startTimer = setTimeout(() => {
      const id = setInterval(() => {
        setVal(base + Math.floor((Math.random() - 0.5) * jitter * 2));
      }, 700);
      return () => clearInterval(id);
    }, delay * 1000);
    return () => clearTimeout(startTimer);
  }, [base, jitter, delay, reduced]);
  return (
    <text
      x={x}
      y={y}
      textAnchor="end"
      fontFamily="var(--font-mono)"
      fontSize="9"
      fill="var(--sev-p0)"
      letterSpacing="0.04em"
    >
      {val.toLocaleString()}/s
    </text>
  );
}

function DiagramAfter() {
  const reduced = useReducedMotion();

  const nodes = [
    {
      y: 8,
      label: "SIGNALS",
      sub: "10K /sec",
      muted: true,
      side: "REST · gRPC · webhooks",
    },
    {
      y: 70,
      label: "DEBOUNCE",
      sub: "redis lua · atomic",
      muted: false,
      side: "100 → 1",
    },
    {
      y: 132,
      label: "WORK ITEMS",
      sub: "pgx · serializable",
      muted: false,
      side: "p99 1.89ms",
    },
    {
      y: 194,
      label: "SRE",
      sub: "dashboard · 2s poll",
      muted: true,
      side: "1 person",
    },
  ];
  return (
    <svg
      viewBox="0 0 360 260"
      xmlns="http://www.w3.org/2000/svg"
      className="h-full w-full"
      aria-hidden
    >
      <defs>
        <marker
          id="arr-active"
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

      {nodes.map((node, i, all) => (
        <g key={node.label}>
          {}
          <rect
            x="80"
            y={node.y}
            width="160"
            height="46"
            rx="6"
            fill="transparent"
            stroke={node.muted ? "var(--diagram-stroke)" : "var(--accent)"}
            strokeWidth="1"
          />
          {}
          <text
            x="160"
            y={node.y + 20}
            textAnchor="middle"
            fontFamily="var(--font-mono)"
            fontSize="11"
            fill={node.muted ? "var(--text-secondary)" : "var(--text-primary)"}
            letterSpacing="0.04em"
          >
            {node.label}
          </text>
          {}
          <text
            x="160"
            y={node.y + 34}
            textAnchor="middle"
            fontFamily="var(--font-mono)"
            fontSize="9"
            fill="var(--text-tertiary)"
            letterSpacing="0.04em"
          >
            {node.sub}
          </text>

          {}
          <text
            x="356"
            y={node.y + 28}
            textAnchor="end"
            fontFamily="var(--font-mono)"
            fontSize="10"
            fill={node.muted ? "var(--text-tertiary)" : "var(--accent)"}
            letterSpacing="0.04em"
          >
            {node.side}
          </text>

          {}
          {i < all.length - 1 && (
            <path
              d={`M 160 ${node.y + 46} L 160 ${all[i + 1].y}`}
              stroke="var(--accent)"
              strokeWidth="1"
              fill="none"
              markerEnd="url(#arr-active)"
            />
          )}
        </g>
      ))}

      {}
      {!reduced && (
        <motion.circle
          r="3"
          fill="var(--accent)"
          cx="160"
          animate={{
            cy: [31, 58, 93, 120, 155, 182, 217, 217],
          }}
          transition={{
            duration: 4,
            repeat: Infinity,
            ease: "easeInOut",
            times: [0, 0.14, 0.28, 0.42, 0.57, 0.71, 0.85, 1],
          }}
        />
      )}
    </svg>
  );
}
