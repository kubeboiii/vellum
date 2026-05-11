// THEME.md §7.1 nav strip.
//
// Anatomy:
//   ▎IMS · ● Live Feed                                  🔔 mute    ⚙
//   ↑                                                   (icons)
//   lime accent bar + lime wordmark + active label
//
// Height: 48px. Left lime accent bar 2px wide. The wordmark is the
// only place "IMS" appears in lime as text (everywhere else, lime is
// reserved for buttons / focus / sparklines).

"use client";

import { IconBellOff, IconSettings } from "@tabler/icons-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

interface NavProps {
  title?: string;
  // muted toggles the bell icon between Bell (sound on) and BellOff.
  // Phase 5 v1: state lives in the parent; we just render. The actual
  // sine-wave beep (§5.2) isn't wired yet; muted always = true for now.
  muted?: boolean;
  onToggleMute?: () => void;
}

// NAV_LINKS are the inline horizontal links. Active route gets a
// lime underline accent per §2.2. We split them into "operational"
// (always visible) and "tools" (visible on wider viewports only)
// so the bar stays readable at small widths.
const NAV_LINKS = [
  { href: "/dashboard", label: "Live", group: "ops" as const },
  { href: "/incidents/closed", label: "History", group: "ops" as const },
  { href: "/postmortem", label: "RCA Queue", group: "ops" as const },
  { href: "/flow", label: "Flow", group: "tools" as const },
  { href: "/simulate", label: "Simulate", group: "tools" as const },
  { href: "/load-test", label: "Load test", group: "tools" as const },
];

export function Nav({ title = "Live Feed", muted = true, onToggleMute }: NavProps) {
  const pathname = usePathname();
  return (
    <nav
      className="flex h-12 items-center justify-between border-b border-border-subtle bg-bg-base px-6"
      aria-label="primary"
    >
      <div className="flex items-center gap-3">
        {/* Lime accent bar — the §7.1 ▎ glyph translated to a div. */}
        <span className="h-5 w-0.5 bg-accent" aria-hidden />
        <Link
          href="/"
          className="font-sans text-page font-semibold tracking-tight text-accent transition-colors duration-fast hover:text-accent-bright"
        >
          IMS
        </Link>
        <span className="font-mono text-text-tertiary" aria-hidden>·</span>
        <span className="font-sans text-card font-medium text-text-primary">
          {title}
        </span>

        {/* Secondary nav links. The active link gets a 1px lime
            underline; inactive ones are bare text. Tools group is
            hidden below md so the small-width bar stays readable. */}
        <ul className="ml-4 flex items-center gap-3 border-l border-border-subtle pl-4">
          {NAV_LINKS.map((l) => {
            const active = pathname === l.href;
            const hide =
              l.group === "tools" ? "hidden md:list-item" : "";
            return (
              <li key={l.href} className={hide}>
                <Link
                  href={l.href}
                  className={`relative font-sans text-meta font-medium uppercase tracking-[0.05em] transition-colors duration-fast ${
                    active ? "text-text-primary" : "text-text-tertiary hover:text-text-secondary"
                  }`}
                >
                  {l.label}
                  {active && (
                    <span
                      className="absolute -bottom-1.5 left-0 right-0 h-px bg-accent"
                      aria-hidden
                    />
                  )}
                </Link>
              </li>
            );
          })}
        </ul>
      </div>

      <div className="flex items-center gap-1.5">
        <button
          type="button"
          onClick={onToggleMute}
          className="rounded-sm p-1.5 text-text-tertiary transition-colors duration-fast hover:bg-bg-elevated hover:text-text-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
          aria-label={muted ? "unmute alerts" : "mute alerts"}
          title={muted ? "Sound off" : "Sound on"}
        >
          <IconBellOff size={16} />
        </button>
        <button
          type="button"
          className="rounded-sm p-1.5 text-text-tertiary transition-colors duration-fast hover:bg-bg-elevated hover:text-text-secondary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40"
          aria-label="settings"
          title="Settings (Phase 7)"
        >
          <IconSettings size={16} />
        </button>
      </div>
    </nav>
  );
}
