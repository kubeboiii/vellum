import { createHighlighter, type Highlighter, type ThemeInput } from "shiki";

import imsDarkJson from "./vellum-dark-theme.json";
const imsDark = imsDarkJson as unknown as ThemeInput;

let highlighter: Highlighter | null = null;

async function getHighlighter(): Promise<Highlighter> {
  if (!highlighter) {
    highlighter = await createHighlighter({
      themes: [imsDark],
      langs: ["go", "lua"],
    });
  }
  return highlighter;
}

export async function highlight(
  code: string,
  lang: "go" | "lua",
): Promise<string> {
  const h = await getHighlighter();
  return h.codeToHtml(code, {
    lang,
    theme: "vellum-dark",
  });
}
