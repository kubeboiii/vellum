"use client";

import Link from "next/link";
import { useState, type KeyboardEvent } from "react";

export interface Tab {
  id: string;
  label: string;
  codeHtml: string;
  p1: string;
  p2: string;
  href: string;
}

interface Props {
  tabs: Tab[];
}

export function CodeTabsClient({ tabs }: Props) {
  const [activeId, setActiveId] = useState(tabs[0].id);
  const active = tabs.find((t) => t.id === activeId) ?? tabs[0];

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
    e.preventDefault();
    const idx = tabs.findIndex((t) => t.id === activeId);
    const next = e.key === "ArrowRight" ? (idx + 1) % tabs.length : (idx - 1 + tabs.length) % tabs.length;
    setActiveId(tabs[next].id);

    const el = document.getElementById(`tab-${tabs[next].id}`);
    el?.focus();
  };

  return (
    <div>
      {}
      <div
        role="tablist"
        aria-label="Architecture snippets"
        className="mb-6 flex flex-wrap gap-2"
        onKeyDown={onKeyDown}
      >
        {tabs.map((t) => {
          const isActive = t.id === activeId;
          return (
            <button
              key={t.id}
              id={`tab-${t.id}`}
              type="button"
              role="tab"
              aria-selected={isActive}
              aria-controls={`panel-${t.id}`}
              tabIndex={isActive ? 0 : -1}
              onClick={() => setActiveId(t.id)}
              className={`inline-flex items-center gap-1.5 rounded-sm border px-3 py-1.5 font-mono text-label uppercase tracking-[0.05em] transition-colors duration-fast ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 ${
                isActive
                  ? "border-border-strong bg-bg-elevated text-text-primary"
                  : "border-border-subtle bg-transparent text-text-secondary hover:bg-bg-elevated"
              }`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${isActive ? "bg-accent" : "border border-border-strong bg-transparent"}`}
                aria-hidden
              />
              {t.label}
            </button>
          );
        })}
      </div>

      {}
      <div
        role="tabpanel"
        id={`panel-${active.id}`}
        aria-labelledby={`tab-${active.id}`}
        className="grid gap-6 lg:grid-cols-2"
      >
        {}
        <div className="min-h-[320px] overflow-hidden rounded-md border border-border-subtle [&_pre]:!m-0 [&_pre]:!bg-bg-surface [&_pre]:!p-5 [&_pre]:!text-data [&_pre]:!leading-[1.6] [&_pre]:overflow-x-auto">
          <div

            dangerouslySetInnerHTML={{ __html: active.codeHtml }}
          />
        </div>

        {}
        <div className="flex flex-col gap-4 p-5">
          <p className="font-sans text-[15px] leading-[1.55] text-text-secondary">
            {active.p1}
          </p>
          <p className="font-sans text-[15px] leading-[1.55] text-text-secondary">
            {active.p2}
          </p>
          <Link
            href={active.href}
            target="_blank"
            rel="noreferrer"
            className="mt-auto font-mono text-meta uppercase tracking-[0.04em] text-accent transition-colors duration-fast hover:text-accent-bright"
          >
            View full file ↗
          </Link>
        </div>
      </div>
    </div>
  );
}
