// Root layout. THEME.md §3.1 + §8.3:
//   - Inter for prose (sans), JetBrains Mono for data (mono)
//   - Dark only — no theme toggle
//   - display: 'swap' so first paint isn't blocked on font download
//
// The CSS variables `--font-sans` and `--font-mono` are consumed by
// tailwind.config.ts (font.sans / font.mono) and by globals.css for
// the body default.

import type { Metadata } from "next";
import { Inter, Instrument_Serif, JetBrains_Mono } from "next/font/google";

import { ToastProvider } from "@/components/Toast";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-sans",
  // THEME.md §8.4: only 400, 500, 600. Never 300 thin, never 700 bold.
  weight: ["400", "500", "600"],
});

const jetbrains = JetBrains_Mono({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-mono",
  weight: ["400", "500"],
});

// LANDING.md §3.2 — Instrument Serif italic. Used ONLY for landing-
// page annotation accents + optionally one emphasis word in the hero
// headline. Never for body text or anywhere else in the app.
const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-serif",
  style: "italic",
  weight: "400",
});

export const metadata: Metadata = {
  title: "IMS — Incident Management",
  description:
    "Incident triage dashboard — atomic debounce, transactional state machine, mandatory RCA.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${inter.variable} ${jetbrains.variable} ${instrumentSerif.variable}`}>
      <body className="bg-bg-base text-text-primary antialiased">
        {/* ToastProvider lives at the root so every page can call
            useToast() without re-wiring. Implementation is a Client
            Component, so importing it here implicitly converts the
            body subtree to client-renderable; Server Components in
            the tree still render server-side, the boundary is
            handled by Next.js. */}
        <ToastProvider>{children}</ToastProvider>
      </body>
    </html>
  );
}
