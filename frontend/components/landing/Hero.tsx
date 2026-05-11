import Link from "next/link";

import { LogTape } from "./LogTape";

export function Hero() {
  return (
    <section

      className="relative flex min-h-[calc(100vh-56px)] flex-col px-6 pb-16 pt-24 sm:pt-32"
    >
      {}
      <div className="flex flex-1 flex-col items-center justify-center">
        <div className="w-full max-w-[880px]">
          {}
          <div className="mb-8 flex justify-center">
            <span className="inline-flex items-center gap-2 rounded-sm border border-accent-border bg-accent-bg px-3 py-1 font-mono text-label uppercase tracking-[0.05em] text-accent">
              <span className="h-1.5 w-1.5 rounded-full bg-accent" aria-hidden />
              Production-grade incident management
            </span>
          </div>

          {}
          <h1 className="text-center text-[44px] font-medium leading-[1.05] tracking-[-0.02em] text-text-primary sm:text-[56px] lg:text-[64px]">
            Ten thousand signals a second.
            <br />
            One incident at a time,{" "}
            <span className="font-serif italic text-text-primary">calmly</span>.
          </h1>

          {}
          <p className="mx-auto mt-6 max-w-[60ch] text-center text-[16px] font-normal leading-[1.55] text-text-secondary sm:text-[18px]">
            A high-throughput ingestion pipeline, Redis-atomic debouncer, and
            stateful incident workflow — built in Go, with a war-room dashboard
            in Next.js.
          </p>

          {}
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            <Link
              href="/dashboard"
              className="inline-flex items-center gap-1.5 rounded-sm bg-accent px-5 py-3 font-sans text-[14px] font-medium text-accent-text transition-colors duration-fast ease-out hover:bg-accent-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-base"
            >
              Open dashboard ›
            </Link>
            <Link
              href="https://github.com/kubeboiii/vellum"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 rounded-sm border border-border-subtle bg-transparent px-5 py-3 font-sans text-[14px] font-medium text-text-primary transition-colors duration-fast ease-out hover:bg-bg-elevated hover:border-border-strong"
            >
              View on GitHub ↗
            </Link>
          </div>
        </div>
      </div>

      {}
      <div className="mt-16 text-center">
        <LogTape />
      </div>
    </section>
  );
}
