// THEME.md §7.2 / PRD FR-7.2 — state-transition timeline.
//
// Vertical timeline of audit rows: dot + label per step. Reused on
// detail + closed-incident pages. The dot color matches the
// to_state's state-pill color so the eye can scan the timeline and
// spot regressions / abnormal patterns at a glance.

import type { StateTransition, Status } from "@/lib/types";

const dotColor: Record<Status, string> = {
  OPEN: "bg-state-open",
  INVESTIGATING: "bg-state-investigating",
  RESOLVED: "bg-state-resolved",
  CLOSED: "bg-state-closed",
};

interface TimelineProps {
  transitions: StateTransition[];
  // empty: rendered text when transitions list is empty (e.g. WI is
  // still OPEN and has no history yet).
  empty?: string;
}

function fmt(iso: string): string {
  return new Date(iso).toLocaleString();
}

export function Timeline({ transitions, empty = "No transitions yet." }: TimelineProps) {
  if (transitions.length === 0) {
    return (
      <div className="px-4 py-3 font-mono text-meta text-text-tertiary">
        {empty}
      </div>
    );
  }
  return (
    <ol className="space-y-0">
      {transitions.map((t, i) => (
        <li
          key={t.id}
          className="relative grid grid-cols-[16px_1fr] gap-3 px-4 py-2"
        >
          {/* Vertical thread — connects the dots. */}
          {i < transitions.length - 1 && (
            <span
              className="absolute left-[20px] top-7 h-full w-px bg-border-subtle"
              aria-hidden
            />
          )}
          {/* Dot keyed to the to_state's color. */}
          <span
            className={`mt-1 h-2 w-2 self-start rounded-full ${dotColor[t.to_state]}`}
            aria-hidden
          />
          <div className="min-w-0">
            <div className="font-mono text-data text-text-primary">
              <span className="text-text-secondary">{t.from_state}</span>
              <span className="mx-1.5 text-text-tertiary" aria-hidden>→</span>
              <span>{t.to_state}</span>
            </div>
            <div className="mt-0.5 flex items-center gap-2 font-mono text-meta text-text-tertiary">
              <span>{fmt(t.created_at)}</span>
              {t.actor && (
                <>
                  <span aria-hidden>·</span>
                  <span>{t.actor}</span>
                </>
              )}
            </div>
            {t.reason && (
              <div className="mt-1 font-sans text-meta text-text-secondary">
                {t.reason}
              </div>
            )}
          </div>
        </li>
      ))}
    </ol>
  );
}
