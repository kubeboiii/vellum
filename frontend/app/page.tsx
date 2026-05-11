// Landing page (/) — root route.
//
// Composed per LANDING.md §4: 10 sections in order. This file does
// nothing but assemble them. Each section is a self-contained
// component under `components/landing/`.
//
// CodeTabs is the only `async` server component in the page — Shiki
// pre-renders the snippet HTML at build/render time, so the whole
// page can be rendered without shipping a syntax-highlighter to the
// browser.

import { Capabilities } from "@/components/landing/Capabilities";
import { CodeTabs } from "@/components/landing/CodeTabs";
import { Comparison } from "@/components/landing/Comparison";
import { Hero } from "@/components/landing/Hero";
import { HowItWorks } from "@/components/landing/HowItWorks";
import { LandingFooter } from "@/components/landing/LandingFooter";
import { LandingNav } from "@/components/landing/LandingNav";
import { PatternCards } from "@/components/landing/PatternCards";
import { ProblemStrip } from "@/components/landing/ProblemStrip";
import { TrustAndClosing } from "@/components/landing/TrustAndClosing";

export default function LandingPage() {
  return (
    <>
      <LandingNav />
      {/* pt-14 = 56px (LandingNav height) so the hero starts below
          the fixed nav. The nav itself fades in a background on
          scroll, so the visual transition isn't abrupt. */}
      <main className="pt-14">
        <Hero />
        <ProblemStrip />
        <Comparison />
        <HowItWorks />
        <PatternCards />
        {/* @ts-expect-error CodeTabs is an async Server Component;
            Next.js's TS plugin sometimes warns about this even though
            it's the supported pattern. */}
        <CodeTabs />
        <Capabilities />
        <TrustAndClosing />
      </main>
      <LandingFooter />
    </>
  );
}
