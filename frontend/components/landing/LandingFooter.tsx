import Link from "next/link";

interface Column {
  header: string;
  links: { label: string; href: string; external?: boolean }[];
}

const COLUMNS: Column[] = [
  {
    header: "Project",
    links: [
      { label: "README", href: "https://github.com/kubeboiii/vellum#readme", external: true },
      { label: "Architecture", href: "https://github.com/kubeboiii/vellum/blob/main/docs/01-architecture.md", external: true },
      { label: "Decisions log", href: "https://github.com/kubeboiii/vellum/blob/main/docs/decisions.md", external: true },
    ],
  },
  {
    header: "Links",
    links: [
      { label: "GitHub ↗", href: "https://github.com/kubeboiii/vellum", external: true },
      { label: "Dashboard", href: "/dashboard" },
      { label: "License", href: "https://github.com/kubeboiii/vellum/blob/main/README.md", external: true },
    ],
  },
];

export function LandingFooter() {
  return (
    <footer className="border-t border-divider bg-bg-base px-6 py-16">
      <div className="mx-auto max-w-[1120px]">
        <div className="grid grid-cols-1 gap-12 sm:grid-cols-3">
          {}
          <div>
            <div className="flex items-center gap-2.5">
              <span className="block h-[18px] w-[3px] bg-accent" aria-hidden />
              <span className="font-sans text-card font-medium tracking-tight text-text-primary">
                Vellum
              </span>
            </div>
            <p className="mt-4 max-w-[28ch] font-sans text-body text-text-secondary">
              Incident management for production teams. A backend-heavy
              build that ingests, debounces, and runs incidents through
              a stateful workflow with mandatory RCA.
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
