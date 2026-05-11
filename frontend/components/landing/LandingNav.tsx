"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

const CENTER_LINKS = [
  { href: "#product", label: "Product" },
  { href: "#architecture", label: "Architecture" },
  { href: "https://github.com/kubeboiii/vellum", label: "GitHub ↗", external: true },
];

export function LandingNav() {

  const [scrolled, setScrolled] = useState(false);

  useEffect(() => {
    function onScroll() {
      setScrolled(window.scrollY > 24);
    }
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <nav
      className={`fixed inset-x-0 top-0 z-50 h-14 transition-colors duration-base ease-out ${
        scrolled
          ? "border-b border-divider bg-bg-surface/95 backdrop-blur-[2px]"
          : "border-b border-transparent bg-transparent"
      }`}
      aria-label="primary"
    >
      <div className="mx-auto flex h-full max-w-[1120px] items-center justify-between px-6">
        {}
        <Link href="/" className="flex items-center gap-2.5">
          <span className="block h-[18px] w-[3px] bg-accent" aria-hidden />
          <span className="font-sans text-card font-medium tracking-tight text-text-primary">
            Vellum
          </span>
        </Link>

        {}
        <ul className="hidden items-center gap-8 sm:flex">
          {CENTER_LINKS.map((l) => (
            <li key={l.href}>
              <Link
                href={l.href}
                target={l.external ? "_blank" : undefined}
                rel={l.external ? "noreferrer" : undefined}
                className="font-mono text-meta uppercase tracking-[0.05em] text-text-secondary transition-colors duration-fast hover:text-text-primary"
              >
                {l.label}
              </Link>
            </li>
          ))}
        </ul>

        {}
        <Link
          href="/dashboard"
          className="inline-flex items-center gap-1 rounded-sm border border-border-subtle bg-transparent px-3 py-1.5 font-sans text-meta font-medium text-text-primary transition-colors duration-fast hover:bg-bg-elevated hover:border-border-strong"
        >
          Open ›
        </Link>
      </div>
    </nav>
  );
}
