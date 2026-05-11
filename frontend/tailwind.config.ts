import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {

        bg: {
          base: "#000000",
          surface: "#0A0A0A",
          elevated: "#141414",
          input: "#0F0F0F",
          hover: "#1A1A1A",
        },
        border: {
          subtle: "#1F1F1F",
          strong: "#2A2A2A",
          focus: "#404040",
        },
        text: {
          primary: "#FAFAFA",
          secondary: "#A1A1AA",
          tertiary: "#71717A",
          disabled: "#52525B",
        },

        accent: {
          DEFAULT: "#BEF264",
          bright: "#D9F99D",
          dim: "#65A30D",
          bg: "#0A1004",
          border: "#365314",
          text: "#1A2E05",
        },

        sev: {
          p0: "#EF4444",
          "p0-bg": "#1A0606",
          "p0-border": "#7F1D1D",
          p1: "#F97316",
          "p1-bg": "#1A0F06",
          "p1-border": "#7C2D12",
          p2: "#F59E0B",
          "p2-bg": "#1A1206",
          "p2-border": "#78350F",
          p3: "#3B82F6",
          "p3-bg": "#06101A",
          "p3-border": "#1E3A8A",
        },
        state: {
          open: "#EF4444",
          investigating: "#F59E0B",
          resolved: "#10B981",
          closed: "#71717A",
        },

        success: "#10B981",
        warning: "#F59E0B",
        danger: "#EF4444",
        info: "#3B82F6",

        annotation: {
          DEFAULT: "#A78BFA",
          dim: "#7C3AED",
        },

        divider: "rgba(255,255,255,0.06)",

        diagram: {
          stroke: "#3F3F46",
          label: "#A1A1AA",
          active: "#BEF264",
          problem: "#EF4444",
        },
      },
      fontFamily: {

        sans: ["var(--font-sans)", "Inter", "system-ui", "sans-serif"],
        mono: ["var(--font-mono)", "JetBrains Mono", "SF Mono", "Menlo", "monospace"],

        serif: ["var(--font-serif)", "Instrument Serif", "Georgia", "serif"],
      },
      fontSize: {

        label: ["11px", { lineHeight: "1.4", letterSpacing: "0.05em" }],
        meta: ["11px", { lineHeight: "1.4" }],
        data: ["12px", { lineHeight: "1.4" }],
        body: ["13px", { lineHeight: "1.5" }],
        card: ["13px", { lineHeight: "1.4" }],
        section: ["15px", { lineHeight: "1.4" }],
        page: ["20px", { lineHeight: "1.3" }],
        stat: ["22px", { lineHeight: "1.2" }],
      },
      borderRadius: {

        sm: "4px",
        md: "6px",
        lg: "8px",
      },
      spacing: {

      },
      transitionTimingFunction: {

        out: "cubic-bezier(0.16, 1, 0.3, 1)",
        "in-out": "cubic-bezier(0.65, 0, 0.35, 1)",
      },
      transitionDuration: {
        fast: "120ms",
        base: "200ms",
        slow: "400ms",
      },
      keyframes: {

        "pulse-p0": {
          "0%, 100%": { boxShadow: "0 0 0 0 rgba(239, 68, 68, 0.45)" },
          "50%": { boxShadow: "0 0 0 6px transparent" },
        },

        "pulse-live": {
          "0%, 100%": { opacity: "1" },
          "50%": { opacity: "0.5" },
        },

        "fade-slide-in": {
          "0%": { opacity: "0", transform: "translateY(-8px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
      },
      animation: {
        "pulse-p0": "pulse-p0 1500ms cubic-bezier(0.65, 0, 0.35, 1) infinite",
        "pulse-live": "pulse-live 3000ms cubic-bezier(0.65, 0, 0.35, 1) infinite",
        "fade-slide-in": "fade-slide-in 300ms cubic-bezier(0.16, 1, 0.3, 1)",
      },
    },
  },
  plugins: [],
};
export default config;
