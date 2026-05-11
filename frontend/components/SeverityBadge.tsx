// THEME.md §6.1 — Severity badge.
//
// Anatomy:
//   - 11px mono, font-medium (500)
//   - severity-keyed text + bg + border
//   - radius-sm (4px), px-2 py-0.5
//   - no letter-spacing change
//
// Color discipline: severity is the *only* place these reds/oranges/
// ambers/blues appear in the UI. Never use them for decoration.

import type { Severity } from "@/lib/types";

const styles: Record<Severity, string> = {
  P0: "text-red-400 bg-sev-p0-bg border-sev-p0-border",
  P1: "text-orange-400 bg-sev-p1-bg border-sev-p1-border",
  P2: "text-amber-400 bg-sev-p2-bg border-sev-p2-border",
  P3: "text-blue-400 bg-sev-p3-bg border-sev-p3-border",
};

export function SeverityBadge({ severity }: { severity: Severity }) {
  return (
    <span
      className={`inline-flex items-center rounded-sm border px-2 py-0.5 font-mono text-label font-medium ${styles[severity]}`}
    >
      {severity}
    </span>
  );
}
