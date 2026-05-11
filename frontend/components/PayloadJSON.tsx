// THEME.md §6.5 — Signal payload syntax highlighting.
//
// Hand-rolled tokenizer (smaller than Prism, no extra dep). Renders:
//   keys: text-secondary
//   strings: emerald
//   numbers: amber
//   booleans / null: blue
// All mono 12px, line-height 1.5. Whitespace preserved.

import React from "react";

// tokenize splits a JSON.stringify-ed string into colorable spans.
// Regex-based, single pass. Not strictly safe for every valid JSON
// edge case (Unicode escapes inside strings render fine; that's the
// only "wonky" case in real signals).
function tokenize(json: string): React.ReactNode[] {
  const re =
    /("(?:[^"\\]|\\.)*"(?:\s*:)?|\b(?:true|false|null)\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;

  const out: React.ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let i = 0;
  while ((match = re.exec(json)) !== null) {
    if (match.index > lastIndex) {
      out.push(json.slice(lastIndex, match.index));
    }
    const token = match[0];
    let cls = "";
    if (token.endsWith(":")) {
      // It's a key. Strip the trailing colon out and render it
      // separately so the punctuation color matches body text.
      cls = "text-text-secondary";
      out.push(
        <span key={i++} className={cls}>
          {token.slice(0, -1)}
        </span>,
      );
      out.push(":");
    } else if (token.startsWith('"')) {
      cls = "text-emerald-400";
      out.push(<span key={i++} className={cls}>{token}</span>);
    } else if (token === "true" || token === "false" || token === "null") {
      cls = "text-blue-400";
      out.push(<span key={i++} className={cls}>{token}</span>);
    } else {
      cls = "text-amber-400";
      out.push(<span key={i++} className={cls}>{token}</span>);
    }
    lastIndex = match.index + token.length;
  }
  if (lastIndex < json.length) out.push(json.slice(lastIndex));
  return out;
}

interface PayloadJSONProps {
  value: unknown;
}

export function PayloadJSON({ value }: PayloadJSONProps) {
  const text = React.useMemo(() => {
    try {
      return JSON.stringify(value, null, 2);
    } catch {
      return String(value);
    }
  }, [value]);
  return (
    <pre className="overflow-x-auto whitespace-pre font-mono text-data leading-[1.5] text-text-primary">
      {tokenize(text)}
    </pre>
  );
}
