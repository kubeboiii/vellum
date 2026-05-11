"use client";

import { useEffect, useState } from "react";

import {
  PERSONA_DESCRIPTIONS,
  PERSONA_LABELS,
  PERSONA_ROLES,
  type Persona,
  readPersona,
  writePersona,
} from "@/lib/persona";

interface Props {

  value: Persona;
  onChange: (next: Persona) => void;
}

const ORDER: Persona[] = ["sre", "commander", "postmortem"];

export function PersonaSwitcher({ value, onChange }: Props) {

  useEffect(() => {
    const stored = readPersona();
    if (stored !== value) onChange(stored);

    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);

  return (
    <section className="rounded-md border border-border-subtle bg-bg-surface p-4">
      <header className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span
            className="h-1.5 w-1.5 animate-pulse-live rounded-full bg-accent"
            aria-hidden
          />
          <h2 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
            View as
          </h2>
        </div>
        <span className="font-mono text-meta text-text-tertiary">
          rearranges the feed
        </span>
      </header>

      <div
        className="grid grid-cols-1 gap-2 sm:grid-cols-3"
        role="radiogroup"
        aria-label="persona view"
      >
        {ORDER.map((p) => {
          const active = mounted && p === value;
          return (
            <button
              key={p}
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => {
                writePersona(p);
                onChange(p);
              }}
              className={`group flex flex-col items-start gap-1 rounded-md border p-3 text-left transition-[border-color,background-color,box-shadow] duration-fast ${
                active
                  ? "border-accent bg-accent-bg shadow-[0_0_24px_-10px_rgba(190,242,100,0.55)]"
                  : "border-border-subtle bg-transparent hover:border-border-strong hover:bg-bg-elevated"
              }`}
            >
              <span className="flex w-full items-center justify-between">
                <span
                  className={`font-sans text-card font-medium ${
                    active ? "text-accent" : "text-text-primary"
                  }`}
                >
                  {PERSONA_LABELS[p]}
                </span>
                {active && (
                  <span
                    className="font-mono text-meta uppercase tracking-[0.05em] text-accent"
                    aria-hidden
                  >
                    active
                  </span>
                )}
              </span>
              <span
                className={`font-mono text-meta uppercase tracking-[0.05em] ${
                  active ? "text-accent" : "text-text-tertiary"
                }`}
              >
                {PERSONA_ROLES[p]}
              </span>
              <span
                className={`font-sans text-meta leading-[1.4] ${
                  active ? "text-text-primary" : "text-text-secondary"
                }`}
              >
                {PERSONA_DESCRIPTIONS[p]}
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
