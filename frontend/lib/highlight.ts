// Server-side Shiki helper. Used by the landing-page Code Tabs to
// pre-render syntax-highlighted HTML.
//
// We use the modern fine-grained API (`createHighlighter` from
// `shiki/bundle/web`) so we only load Go + Lua grammars + our custom
// theme — about 80 KB on the server. The resulting HTML strings are
// inlined into the rendered page; no Shiki runtime ships to the
// browser.

import { createHighlighter, type Highlighter, type ThemeInput } from "shiki";

// Cast: the JSON file has the same shape Shiki expects (its
// ThemeInput is a TextMate-style theme, which is what
// ims-dark-theme.json conforms to), but TS can't statically
// verify the `type: "dark"` literal narrowing. The cast keeps
// the build clean without weakening the runtime contract.
import imsDarkJson from "./ims-dark-theme.json";
const imsDark = imsDarkJson as unknown as ThemeInput;

// Cached singleton highlighter. createHighlighter is expensive
// (loads grammar JSONs); reusing across calls in dev/HMR matters.
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

/** Highlight a code snippet and return HTML ready to dangerouslySetInnerHTML. */
export async function highlight(
  code: string,
  lang: "go" | "lua",
): Promise<string> {
  const h = await getHighlighter();
  return h.codeToHtml(code, {
    lang,
    theme: "ims-dark",
  });
}
