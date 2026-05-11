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
      {}
      <main className="pt-14">
        <Hero />
        <ProblemStrip />
        <Comparison />
        <HowItWorks />
        <PatternCards />
        {}
        <CodeTabs />
        <Capabilities />
        <TrustAndClosing />
      </main>
      <LandingFooter />
    </>
  );
}
