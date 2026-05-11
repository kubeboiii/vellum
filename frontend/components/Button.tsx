// THEME.md §6.9 — Button primitive (primary + ghost variants).
//
// `primary` = lime fill; the high-conviction button. Use SPARINGLY
// (one per page). Canonical example: "Submit & Close" on the RCA form.
//
// `ghost` = transparent background with hairline border; for
// secondary actions. State-advance buttons on the detail page use
// this variant.
//
// Both follow THEME.md weight discipline (500, not 600) and use
// radius-sm. Active state scales 0.97 for tactile feedback.

import type { ButtonHTMLAttributes, ReactNode } from "react";

type Variant = "primary" | "ghost" | "danger";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant;
  children: ReactNode;
}

const base =
  "inline-flex items-center justify-center gap-1.5 rounded-sm px-3 py-1.5 font-sans text-body font-medium transition-[background-color,color,border-color,box-shadow,transform] duration-fast ease-out " +
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40 focus-visible:ring-offset-2 focus-visible:ring-offset-bg-base " +
  "active:scale-[0.97] disabled:cursor-not-allowed disabled:bg-zinc-800 disabled:text-zinc-600 disabled:active:scale-100 disabled:hover:shadow-none";

const variants: Record<Variant, string> = {
  // Lime fill. Text is dark lime (--accent-text), NEVER white.
  // Hover gets the same lime halo we use on landing-page CTAs so the
  // "Submit & Close" button on the RCA form lands as the decisive
  // action on the page.
  primary:
    "bg-accent text-accent-text hover:bg-accent-bright active:bg-accent-dim hover:shadow-[0_0_24px_-6px_rgba(190,242,100,0.55)]",
  // Transparent over the surface; hairline border.
  ghost: "border border-border-subtle bg-transparent text-text-primary hover:bg-bg-elevated hover:border-border-strong",
  // Red destructive variant — currently unused but reserved for
  // anything that's irreversible (e.g. force-close).
  danger: "border border-sev-p0-border bg-sev-p0-bg text-red-300 hover:bg-sev-p0-border hover:text-text-primary",
};

export function Button({
  variant = "ghost",
  className = "",
  children,
  ...rest
}: ButtonProps) {
  return (
    <button className={`${base} ${variants[variant]} ${className}`} {...rest}>
      {children}
    </button>
  );
}
