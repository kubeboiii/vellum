"use client";

interface QueueGaugeProps {
  depth: number;
  capacity: number;
}

export function QueueGauge({ depth, capacity }: QueueGaugeProps) {
  const safeCap = Math.max(1, capacity);
  const pct = Math.min(100, (depth / safeCap) * 100);

  const tone =
    pct > 85 ? "red" : pct > 60 ? "amber" : "lime";
  const fill =
    tone === "red"
      ? "bg-sev-p0"
      : tone === "amber"
        ? "bg-sev-p1"
        : "bg-accent";
  const labelColor =
    tone === "red"
      ? "text-sev-p0"
      : tone === "amber"
        ? "text-sev-p1"
        : "text-text-secondary";
  return (
    <div className="flex items-center gap-2">
      <span className="font-mono text-meta uppercase tracking-[0.05em] text-text-tertiary">
        Queue
      </span>
      <div
        className="relative h-1.5 w-[120px] overflow-hidden rounded-sm bg-bg-elevated"
        role="meter"
        aria-valuenow={depth}
        aria-valuemin={0}
        aria-valuemax={capacity}
        aria-label="ingestion queue depth"
      >
        <div
          className={`h-full ${fill} ${tone === "red" ? "animate-pulse-live" : ""}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className={`font-mono text-meta tabular-nums ${labelColor}`}>
        {depth.toLocaleString()}/{capacity.toLocaleString()}
      </span>
    </div>
  );
}
