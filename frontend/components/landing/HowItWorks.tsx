"use client";

import {
  IconAlertTriangle,
  IconApi,
  IconBell,
  IconBolt,
  IconBox,
  IconCloud,
  IconDatabase,
  IconHistory,
  IconLayoutDashboard,
  IconStack2,
} from "@tabler/icons-react";
import { motion, useInView, useReducedMotion } from "framer-motion";
import { useEffect, useRef, useState } from "react";

type TablerIcon = typeof IconApi;

interface CardSpec {
  Icon: TablerIcon;
  label: string;
  sub: string;
  metric?: string;
}

const SIGNAL_CARDS: CardSpec[] = [
  { Icon: IconApi, label: "APIs", sub: "REST · gRPC · webhooks" },
  { Icon: IconBolt, label: "Caches", sub: "Redis · Memcached" },
  { Icon: IconStack2, label: "Queues", sub: "Kafka · RabbitMQ" },
  { Icon: IconDatabase, label: "Databases", sub: "Postgres · Mongo" },
];

interface IMSCardSpec extends CardSpec {
  callout: string;
}

const IMS_CARDS: IMSCardSpec[] = [
  {
    Icon: IconCloud,
    label: "Ingestion API",
    sub: "Gin · gRPC bidi · token-bucket per source",
    metric: "10K /sec",
    callout: "non-blocking enqueue · 503 on full",
  },
  {
    Icon: IconBox,
    label: "Debouncer",
    sub: "Redis Lua · EVALSHA · 10s window",
    metric: "100 → 1",
    callout: "atomic check-then-act, single round-trip",
  },
  {
    Icon: IconAlertTriangle,
    label: "Workflow + RCA",
    sub: "pgx · SERIALIZABLE · State pattern",
    metric: "MTTR auto",
    callout: "RCA required for CLOSED · audited",
  },
];

const OUTPUT_CARDS: CardSpec[] = [
  { Icon: IconLayoutDashboard, label: "Dashboard", sub: "Next.js · 2s polling" },
  { Icon: IconBell, label: "Alerts", sub: "PagerDuty · Slack · Console" },
  { Icon: IconHistory, label: "Audit trail", sub: "state_transitions · RCA" },
];

const HEADLINE_METRICS = [
  { value: "10K", unit: "signals/sec" },
  { value: "50K", unit: "channel depth" },
  { value: "100", unit: "per window" },
  { value: "<2ms", unit: "p99" },
];

function SystemStatusStrip() {
  const reduced = useReducedMotion();
  const [rate, setRate] = useState(8421);
  useEffect(() => {
    if (reduced) return;
    const id = setInterval(() => {
      setRate(8400 + Math.floor(Math.random() * 600));
    }, 1500);
    return () => clearInterval(id);
  }, [reduced]);
  return (
    <div
      className="mx-auto mb-12 flex max-w-3xl flex-wrap items-center justify-between gap-x-6 gap-y-2 rounded-md border border-border-subtle bg-bg-surface px-4 py-2 font-mono text-meta uppercase tracking-[0.05em] text-text-secondary"
      suppressHydrationWarning
    >
      <div className="flex items-center gap-2">
        <span className="block h-3 w-[3px] bg-accent" aria-hidden />
        <span className="text-text-primary">Vellum</span>
        <span className="text-text-tertiary">·</span>
        <span>v1.0</span>
      </div>
      <div className="flex items-center gap-4">
        <span className="flex items-center gap-1.5">
          <span className="inline-block h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent" aria-hidden />
          <span className="text-accent">health 200</span>
        </span>
        <span>uptime 14d</span>
        <span className="tabular-nums text-text-primary">
          {rate.toLocaleString()}/s
        </span>
      </div>
    </div>
  );
}

function GridTexture({ tone }: { tone: "muted" | "active" }) {
  const reduced = useReducedMotion();
  const stroke =
    tone === "active"
      ? "rgba(190,242,100,0.06)"
      : "rgba(255,255,255,0.025)";
  return (
    <svg
      className="pointer-events-none absolute inset-0 h-full w-full"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden
    >
      <defs>
        <pattern
          id={`grid-${tone}`}
          width="32"
          height="32"
          patternUnits="userSpaceOnUse"
        >
          <path
            d="M 32 0 L 0 0 0 32"
            fill="none"
            stroke={stroke}
            strokeWidth="1"
          />
        </pattern>
      </defs>
      <motion.rect
        x={-32}
        y={-32}
        width="200%"
        height="200%"
        fill={`url(#grid-${tone})`}
        initial={{ x: -32, y: -32 }}
        animate={reduced ? undefined : { x: 0, y: 0 }}
        transition={
          reduced
            ? undefined
            : {
                duration: 18,
                repeat: Infinity,
                repeatType: "reverse",
                ease: "linear",
              }
        }
      />
    </svg>
  );
}

function NeonNumeral({ value, tone }: { value: string; tone: "muted" | "active" }) {
  const color = tone === "active" ? "var(--accent)" : "var(--text-tertiary)";
  const reduced = useReducedMotion();
  return (
    <motion.svg
      className="pointer-events-none absolute right-4 top-4 h-[72px] w-[100px] sm:right-6 sm:top-6 sm:h-[80px] sm:w-[112px]"
      viewBox="0 0 112 80"
      xmlns="http://www.w3.org/2000/svg"
      initial={{ opacity: 0 }}
      whileInView={{ opacity: 1 }}
      viewport={{ once: true, margin: "-15%" }}
      transition={{ duration: 0.6, ease: "easeOut" }}
      aria-hidden
    >
      <defs>
        <filter id={`glow-${value}`} x="-50%" y="-50%" width="200%" height="200%">
          <feGaussianBlur stdDeviation="3" />
        </filter>
      </defs>
      {}
      <motion.text
        x="56"
        y="62"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="60"
        fontWeight="500"
        fill={color}
        opacity={tone === "active" ? 0.4 : 0.2}
        filter={`url(#glow-${value})`}
        animate={
          reduced || tone !== "active"
            ? undefined
            : { opacity: [0.3, 0.55, 0.3] }
        }
        transition={
          reduced ? undefined : { duration: 3.4, repeat: Infinity, ease: "easeInOut" }
        }
      >
        {value}
      </motion.text>
      {}
      <text
        x="56"
        y="62"
        textAnchor="middle"
        fontFamily="var(--font-mono)"
        fontSize="60"
        fontWeight="500"
        fill={color}
        opacity={tone === "active" ? 0.7 : 0.4}
      >
        {value}
      </text>
    </motion.svg>
  );
}

function AccentScanner() {
  const reduced = useReducedMotion();
  return (
    <span
      className="relative block h-[14px] w-[3px] overflow-hidden bg-accent/20"
      aria-hidden
    >
      <motion.span
        className="absolute left-0 right-0 h-1.5 bg-accent"
        initial={{ top: 0 }}
        animate={reduced ? undefined : { top: ["0%", "60%", "0%"] }}
        transition={
          reduced
            ? undefined
            : { duration: 1.8, repeat: Infinity, ease: "easeInOut" }
        }
      />
    </span>
  );
}

function MetricChip({ value }: { value: string }) {
  const reduced = useReducedMotion();
  const ref = useRef<HTMLSpanElement>(null);
  const inView = useInView(ref, { once: true, margin: "-20%" });

  const match = value.match(/^(\d+)([KM]?)(.*)$/);
  const target = match ? parseInt(match[1], 10) : null;
  const suffix = match ? `${match[2]}${match[3]}` : "";

  const [display, setDisplay] = useState<string>(
    target !== null && !reduced && !inView ? `0${suffix}` : value,
  );

  useEffect(() => {
    if (target === null || reduced || !inView) {
      setDisplay(value);
      return;
    }
    let raf = 0;
    const start = performance.now();
    const duration = 800;
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      const eased = 1 - Math.pow(1 - t, 4);
      const n = Math.round(target * eased);
      setDisplay(`${n}${suffix}`);
      if (t < 1) raf = requestAnimationFrame(tick);
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
  }, [target, suffix, value, reduced, inView]);

  return (
    <span
      ref={ref}
      className="ml-auto rounded-sm border border-border-subtle bg-bg-base px-1.5 py-0.5 font-mono text-[10px] uppercase tracking-[0.04em] tabular-nums text-text-secondary"
    >
      {display}
    </span>
  );
}

function SourceCard({ Icon, label, sub }: CardSpec) {
  return (
    <motion.div
      whileHover={{ y: -2 }}
      transition={{ duration: 0.2 }}
      className="group flex items-start gap-3 rounded-md border border-border-subtle bg-bg-base/80 px-5 py-4 transition-all duration-base hover:border-border-strong hover:shadow-[0_0_24px_-10px_var(--accent-glow)]"
    >
      {}
      <Icon
        size={14}
        className="mt-1 shrink-0 text-text-secondary transition-colors group-hover:text-accent"
        aria-hidden
      />
      {}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="font-mono text-data uppercase tracking-[0.04em] text-text-primary">
            {label}
          </span>
          <span
            className="inline-block h-1 w-1 shrink-0 animate-pulse-live rounded-full bg-accent/60 group-hover:bg-accent"
            aria-hidden
          />
        </div>
        {}
        <div className="mt-1.5 overflow-hidden text-ellipsis whitespace-nowrap font-mono text-[10px] text-text-tertiary">
          {sub}
        </div>
      </div>
    </motion.div>
  );
}

function IMSCard({ Icon, label, sub, metric, callout }: IMSCardSpec) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 12 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-15%" }}
      transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
      whileHover={{ scale: 1.01 }}
      className="group rounded-md border border-border-subtle bg-bg-base/90 px-5 py-4 backdrop-blur-[1px] transition-all duration-base hover:border-annotation-dim hover:shadow-[0_0_32px_-12px_rgba(167,139,250,0.5)]"
    >
      <div className="flex items-center gap-3">
        <Icon
          size={16}
          className="text-text-secondary transition-colors group-hover:text-annotation"
          aria-hidden
        />
        <span className="font-mono text-data uppercase tracking-[0.04em] text-text-primary">
          {label}
        </span>
        {metric && <MetricChip value={metric} />}
      </div>
      <div className="mt-2 font-mono text-meta text-text-tertiary">{sub}</div>
      <div className="mt-3 flex items-center gap-1.5 border-t border-border-subtle pt-2 font-mono text-meta italic text-annotation">
        <span className="text-annotation/70" aria-hidden>
          ✦
        </span>
        {callout}
      </div>
    </motion.div>
  );
}

function FlowArrow() {
  const reduced = useReducedMotion();
  return (
    <div className="relative flex h-14 justify-center">
      <div className="h-full w-px bg-accent/30" aria-hidden />
      {!reduced && (
        <>
          {}
          <motion.div
            className="absolute left-1/2 h-6 w-px -translate-x-1/2 bg-accent"
            initial={{ top: "-25%" }}
            animate={{ top: "100%" }}
            transition={{ duration: 2.2, repeat: Infinity, ease: "linear" }}
            aria-hidden
          />
          {}
          <motion.div
            className="absolute left-1/2 h-1.5 w-1.5 -translate-x-1/2 rounded-full bg-accent shadow-[0_0_8px_var(--accent-glow)]"
            initial={{ top: 0, opacity: 0 }}
            animate={{ top: ["0%", "100%"], opacity: [0, 1, 1, 0] }}
            transition={{
              duration: 1.6,
              repeat: Infinity,
              ease: "easeInOut",
              times: [0, 0.1, 0.9, 1],
            }}
            aria-hidden
          />
        </>
      )}
      <svg
        width="10"
        height="10"
        viewBox="0 0 10 10"
        className="absolute -bottom-px left-1/2 -translate-x-1/2"
        aria-hidden
      >
        <path d="M0,0 L10,0 L5,8 z" fill="var(--accent)" />
      </svg>
    </div>
  );
}

function MiniFlowArrow() {
  const reduced = useReducedMotion();
  return (
    <div className="relative my-2 flex h-6 justify-center">
      <div className="h-full w-px bg-accent/30" aria-hidden />
      {!reduced && (
        <>
          <motion.div
            className="absolute left-1/2 h-3 w-px -translate-x-1/2 bg-accent"
            initial={{ top: "-50%" }}
            animate={{ top: "100%" }}
            transition={{ duration: 1.6, repeat: Infinity, ease: "linear" }}
            aria-hidden
          />
          <motion.div
            className="absolute left-1/2 h-1 w-1 -translate-x-1/2 rounded-full bg-accent"
            initial={{ top: 0 }}
            animate={{ top: ["0%", "100%"] }}
            transition={{ duration: 1.4, repeat: Infinity, ease: "easeInOut" }}
            aria-hidden
          />
        </>
      )}
    </div>
  );
}

interface RegionProps {
  stage: string;
  label: string;
  subtitle: string;
  variant: "muted" | "active";
  children: React.ReactNode;
}

function Region({ stage, label, subtitle, variant, children }: RegionProps) {
  const isActive = variant === "active";
  return (
    <motion.div
      initial={{ opacity: 0, y: 16 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "-10%" }}
      transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
      className={`relative overflow-hidden rounded-lg border p-8 sm:p-12 ${
        isActive
          ? "border-accent-border bg-accent-bg shadow-[inset_0_0_80px_-20px_var(--accent-glow)]"
          : "border-border-subtle bg-bg-surface"
      }`}
    >
      {}
      <GridTexture tone={variant} />

      {}
      <NeonNumeral value={stage} tone={variant} />

      {}
      <header className="relative mb-8 flex flex-wrap items-center gap-x-4 gap-y-2">
        <div className="flex items-center gap-2">
          {isActive ? (
            <AccentScanner />
          ) : (
            <span
              className="block h-[14px] w-[3px] bg-text-tertiary/30"
              aria-hidden
            />
          )}
          <span
            className={`font-mono text-label uppercase tracking-[0.05em] ${
              isActive ? "text-accent" : "text-text-secondary"
            }`}
          >
            {label}
          </span>
        </div>
        <span className="font-sans text-meta text-text-secondary">
          {subtitle}
        </span>
      </header>

      {}
      <div className="relative">{children}</div>
    </motion.div>
  );
}

export function HowItWorks() {
  return (
    <section
      id="architecture"
      className="border-t border-divider px-6 py-24 sm:py-32"
    >
      <div className="mx-auto max-w-[1120px]">
        <header className="mb-12 text-center">
          <p className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            How it works
          </p>
          <h2 className="mt-3 font-sans text-[28px] font-medium leading-[1.2] text-text-primary sm:text-[36px]">
            Three stages. One pipeline.
          </h2>

          <ul className="mx-auto mt-8 flex max-w-3xl flex-wrap items-center justify-center gap-x-8 gap-y-3 border-y border-divider py-4 font-mono">
            {HEADLINE_METRICS.map((m) => (
              <li key={m.unit} className="flex items-baseline gap-2">
                <span className="text-data font-medium text-accent">
                  {m.value}
                </span>
                <span className="text-[10px] uppercase tracking-[0.05em] text-text-tertiary">
                  {m.unit}
                </span>
              </li>
            ))}
          </ul>
        </header>

        <SystemStatusStrip />

        <Region
          stage="01"
          label="Signals"
          subtitle="Heterogeneous failure signals from across the stack"
          variant="muted"
        >
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {SIGNAL_CARDS.map((c) => (
              <SourceCard key={c.label} {...c} />
            ))}
          </div>
        </Region>

        <FlowArrow />

        <Region
          stage="02"
          label="Vellum"
          subtitle="Atomic debounce, transactional workflow, mandatory RCA"
          variant="active"
        >
          <div className="mx-auto flex max-w-[440px] flex-col gap-0">
            <IMSCard {...IMS_CARDS[0]} />
            <MiniFlowArrow />
            <IMSCard {...IMS_CARDS[1]} />
            <MiniFlowArrow />
            <IMSCard {...IMS_CARDS[2]} />
          </div>
        </Region>

        <FlowArrow />

        <Region
          stage="03"
          label="Output"
          subtitle="What humans and downstream systems see"
          variant="muted"
        >
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
            {OUTPUT_CARDS.map((c) => (
              <SourceCard key={c.label} {...c} />
            ))}
          </div>
        </Region>
      </div>
    </section>
  );
}
