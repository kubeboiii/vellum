// Payload fingerprinting — a stable shape descriptor for a signal's
// `payload` JSON. Two signals with the same top-level key set
// produce the same fingerprint. This lets the detail page collapse
// "200 signals" into "187× `{error,host}`, 13 others" so the SRE
// scans shape, not noise.
//
// The fingerprint is intentionally shallow (top-level keys only).
// Anything deeper would require either tree-walking (slow on 5K
// signals) or schema inference (out of scope). Shallow keys cover
// 90% of the real-world dedupe wins.

import type { Signal } from "./types";

// fingerprint returns a deterministic, human-readable string from
// a payload's top-level key set. Null / non-object payloads bucket
// into "(non-object)". Empty objects become "(empty)".
export function fingerprint(payload: unknown): string {
  if (payload === null || payload === undefined) return "(null)";
  if (typeof payload !== "object") return "(non-object)";
  // Array payloads — keyed by their tag, not their length, so two
  // arrays with same value-shape land in the same bucket.
  if (Array.isArray(payload)) return "(array)";
  const keys = Object.keys(payload as Record<string, unknown>);
  if (keys.length === 0) return "(empty)";
  return "{" + [...keys].sort().join(",") + "}";
}

export interface FingerprintGroup {
  fp: string;
  count: number;
  // exampleSignalId is the first signal we saw with this fingerprint;
  // useful for a "show me an example" link.
  exampleSignalId: string;
}

// groupByFingerprint returns groups sorted by count desc.
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
