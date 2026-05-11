"use client";

import { groupByFingerprint } from "@/lib/fingerprint";
import type { Signal } from "@/lib/types";

interface Props {
  signals: Signal[];
  onSelect?: (fp: string | null) => void;
  selected?: string | null;
}

export function PayloadFingerprints({ signals, onSelect, selected }: Props) {
  if (signals.length === 0) {
    return null;
  }
  const groups = groupByFingerprint(signals);
  const total = signals.length;
  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-2 flex items-center justify-between">
        <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
          Payload fingerprints
        </h3>
        <span className="font-mono text-meta text-text-tertiary tabular-nums">
          {groups.length} {groups.length === 1 ? "shape" : "shapes"}
        </span>
      </header>
      <ul className="space-y-1.5">
        {groups.map((g) => {
          const pct = (g.count / total) * 100;
          const active = selected === g.fp;
          return (
            <li key={g.fp}>
              <button
                type="button"
                onClick={() => onSelect?.(active ? null : g.fp)}
                className={`group flex w-full items-center gap-3 rounded-sm border px-2 py-1 text-left transition-colors duration-fast ${
                  active
                    ? "border-accent bg-accent-bg"
                    : "border-border-subtle bg-transparent hover:border-border-strong hover:bg-bg-elevated"
                }`}
              >
                <span
                  className={`truncate font-mono text-meta ${active ? "text-accent" : "text-text-primary"}`}
                  title={g.fp}
                >
                  {g.fp}
                </span>
                <div className="ml-auto flex shrink-0 items-center gap-2">
                  <div className="h-1 w-16 overflow-hidden rounded-sm bg-bg-elevated">
                    <div
                      className="h-full bg-accent"
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className="w-14 text-right font-mono text-meta tabular-nums text-text-secondary">
                    {g.count}× · {pct.toFixed(0)}%
                  </span>
                </div>
              </button>
            </li>
          );
        })}
      </ul>
      {onSelect && selected && (
        <p className="mt-2 font-mono text-meta text-text-tertiary">
          Filtering raw signals to{" "}
          <span className="text-accent">{selected}</span>. Click again to
          clear.
        </p>
      )}
    </section>
  );
}
