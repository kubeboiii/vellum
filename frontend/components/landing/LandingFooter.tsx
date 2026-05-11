// LANDING.md §5.10 — landing footer.
//
// Three columns at desktop, single column on mobile. Bottom strip
// with a hairline divider above shows the "built solo · year" line
// and a fake-but-honest version number.

import Link from "next/link";

interface Column {
  header: string;
  links: { label: string; href: string; external?: boolean }[];
}

const COLUMNS: Column[] = [
  {
    header: "Project",
    links: [
      { label: "README", href: "https://github.com/kubeboiii/ims#readme", external: true },
      { label: "Architecture", href: "https://github.com/kubeboiii/ims/blob/main/docs/01-architecture.md", external: true },
      { label: "Decisions log", href: "https://github.com/kubeboiii/ims/blob/main/docs/decisions.md", external: true },
    ],
  },
  {
    header: "Links",
    links: [
      { label: "GitHub ↗", href: "https://github.com/kubeboiii/ims", external: true },
      { label: "Dashboard", href: "/dashboard" },
      { label: "License", href: "https://github.com/kubeboiii/ims/blob/main/README.md", external: true },
    ],
  },
];

export function LandingFooter() {
  return (
    <footer className="border-t border-divider bg-bg-base px-6 py-16">
      <div className="mx-auto max-w-[1120px]">
        <div className="grid grid-cols-1 gap-12 sm:grid-cols-3">
          {/* Brand column */}
          <div>
            <div className="flex items-center gap-2.5">
              <span className="block h-[18px] w-[3px] bg-accent" aria-hidden />
              <span className="font-sans text-card font-medium tracking-tight text-text-primary">
                IMS
              </span>
            </div>
            <p className="mt-4 max-w-[28ch] font-sans text-body text-text-secondary">
              Incident Management System. A backend-heavy demo of
              ingestion, debouncing, and stateful incident workflow.
            </p>
          </div>

          {COLUMNS.map((col) => (
            <div key={col.header}>
              <h3 className="font-mono text-label uppercase tracking-[0.05em] text-text-secondary">
                {col.header}
              </h3>
              <ul className="mt-4 space-y-2">
                {col.links.map((l) => (
                  <li key={l.label}>
                    <Link
                      href={l.href}
                      target={l.external ? "_blank" : undefined}
                      rel={l.external ? "noreferrer" : undefined}
                      className="font-sans text-body text-text-secondary transition-colors duration-fast hover:text-text-primary hover:underline"
                    >
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <div className="mt-12 flex flex-wrap items-center justify-between gap-2 border-t border-divider pt-6 font-mono text-label uppercase tracking-[0.05em] text-text-tertiary">
          <span>Built solo · 2026</span>
          <span>v1.0.0</span>
        </div>
      </div>
    </footer>
  );
}
