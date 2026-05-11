import type { Metadata } from "next";
import { Inter, Instrument_Serif, JetBrains_Mono } from "next/font/google";

import { ToastProvider } from "@/components/Toast";
import "./globals.css";

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-sans",

  weight: ["400", "500", "600"],
});

const jetbrains = JetBrains_Mono({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-mono",
  weight: ["400", "500"],
});

const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  display: "swap",
  variable: "--font-serif",
  style: "italic",
  weight: "400",
});

export const metadata: Metadata = {
  title: "Vellum — Incident Management",
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
        {}
        <ToastProvider>{children}</ToastProvider>
      </body>
    </html>
  );
}
