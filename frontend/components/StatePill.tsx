import type { Status } from "@/lib/types";

const pillStyles: Record<Status, { dot: string; pill: string }> = {
  OPEN: {
    dot: "bg-state-open",
    pill: "border-sev-p0-border bg-sev-p0-bg/40 text-red-300",
  },
  INVESTIGATING: {
    dot: "bg-state-investigating",
    pill: "border-sev-p2-border bg-sev-p2-bg/40 text-amber-300",
  },
  RESOLVED: {
    dot: "bg-state-resolved",
    pill: "border-emerald-900 bg-emerald-950/40 text-emerald-300",
  },
  CLOSED: {
    dot: "bg-state-closed",
    pill: "border-zinc-800 bg-zinc-900/50 text-zinc-400",
  },
};

interface StatePillProps {
  status: Status;

  pulseDot?: boolean;
}

export function StatePill({ status, pulseDot }: StatePillProps) {
  const s = pillStyles[status];
  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-sm border px-2 py-0.5 font-mono text-label font-medium uppercase tracking-[0.04em] ${s.pill}`}
    >
      <span

        className={`inline-block h-1.5 w-1.5 rounded-full ${s.dot} ${pulseDot ? "animate-pulse-p0" : ""}`}
        aria-hidden
      />
      {status}
    </span>
  );
}
