// LANDING.md §5.2 — hero section.
//
// Pre-headline pill → two-line H1 with one italic serif word → subhead
// → two CTAs → log-tape at the bottom.
//
// Min-height calc(100vh - 56px) so the hero owns the first viewport.
// Centered vertically via grid. The serif italic emphasis is "calmly"
// at the end of line 2 — single deliberate moment.
//
// LANDING.md §6.3 calls for an annotation here ("live demo — try the
// simulator"). We removed it because the hand-drawn arrow couldn't
// reliably anchor to the primary CTA across viewport widths — the
// label drifted and the alignment looked off (see decisions.md
// 2026-05-11 "Phase 5 polish: removed landing annotations"). The
// serif font is still loaded and used inside the H1.

import Link from "next/link";

import { LogTape } from "./LogTape";

export function Hero() {
  return (
    <section
      // Flex column layout: the main content auto-centers via the
      // flex-1 spacer between it and the terminal. The terminal is
      // a real flow child at the bottom (not absolute), so it can
      // never overlap the CTAs no matter how tall it grows.
      // min-h ensures the hero owns the first viewport (minus the
      // 56px nav). Generous bottom padding keeps the terminal off
      // the section divider.
      className="relative flex min-h-[calc(100vh-56px)] flex-col px-6 pb-16 pt-24 sm:pt-32"
    >
      {/* Flex-1 spacer that pushes content to its natural center. */}
      <div className="flex flex-1 flex-col items-center justify-center">
        <div className="w-full max-w-[880px]">
          {/* Pre-headline pill — single lime dot + uppercase label. */}
          <div className="mb-8 flex justify-center">
            <span className="inline-flex items-center gap-2 rounded-sm border border-accent-border bg-accent-bg px-3 py-1 font-mono text-label uppercase tracking-[0.05em] text-accent">
              <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden />
              Production-grade incident management
            </span>
          </div>

          {/* H1 — two lines, sans, weight 500. One italic serif word
              in line 2 for the deliberate moment. */}
          <h1 className="text-center text-[44px] font-medium leading-[1.05] tracking-[-0.02em] text-text-primary sm:text-[56px] lg:text-[64px]">
            Ten thousand signals a second.
            <br />
            One incident at a time,{" "}
            <span className="font-serif italic text-text-primary">calmly</span>.
          </h1>

          {/* Subhead. max-width 60ch keeps the prose readable. */}
          <p className="mx-auto mt-6 max-w-[60ch] text-center text-[16px] font-normal leading-[1.55] text-text-secondary sm:text-[18px]">
            A high-throughput ingestion pipeline, Redis-atomic debouncer, and
            stateful incident workflow — built in Go, with a war-room dashboard
            in Next.js.
          </p>

          {/* CTAs */}
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-1.5 rounded-sm bg-accent px-5 py-3 font-sans text-[14px] font-medium text-accent-text transition-colors duration-fast ease-out hover:bg-accent-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-base"
            >
              Open dashboard ›
            </Link>
            <Link
              href="https://github.com/kubeboiii/ims"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 rounded-sm border border-border-subtle bg-transparent px-5 py-3 font-sans text-[14px] font-medium text-text-primary transition-colors duration-fast ease-out hover:bg-bg-elevated hover:border-border-strong"
            >
              View on GitHub ↗
            </Link>
          </div>
        </div>
      </div>

      {/* Log tape — real flow child at the bottom. Generous top
          margin keeps it visibly separate from the CTAs. */}
      <div className="mt-16 text-center">
        <LogTape />
      </div>
    </section>
  );
}
