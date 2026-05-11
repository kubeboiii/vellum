import type { Signal } from "./types";

export function fingerprint(payload: unknown): string {
  if (payload === null || payload === undefined) return "(null)";
  if (typeof payload !== "object") return "(non-object)";

  if (Array.isArray(payload)) return "(array)";
  const keys = Object.keys(payload as Record<string, unknown>);
  if (keys.length === 0) return "(empty)";
  return "{" + [...keys].sort().join(",") + "}";
}

export interface FingerprintGroup {
  fp: string;
  count: number;

  exampleSignalId: string;
}

export function groupByFingerprint(signals: Signal[]): FingerprintGroup[] {
  const m = new Map<string, FingerprintGroup>();
  for (const s of signals) {
    const fp = fingerprint(s.payload);
    const g = m.get(fp);
    if (g) {
      g.count++;
    } else {
      m.set(fp, { fp, count: 1, exampleSignalId: s.signal_id });
    }
  }
  return Array.from(m.values()).sort((a, b) => b.count - a.count);
}
