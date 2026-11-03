import type { ReactNode } from "react";

/** Text with inline `code` drawn the way the Markdown editor draws it; backticks never show. */
export function InlineCodeText({ text }: { text: string }) {
  const nodes: ReactNode[] = [];
  let offset = 0;
  for (const part of text.split(/(`[^`]+`)/)) {
    // A segment's position in the text is its stable identity, even when two segments match.
    const key = `${offset}:${part}`;
    nodes.push(
      part.length > 1 && part.startsWith("`") && part.endsWith("`") ? (
        <code className="rounded-xxs bg-sunken/40 px-1 font-mono text-[0.92em]" key={key}>
          {part.slice(1, -1)}
        </code>
      ) : (
        <span key={key}>{part}</span>
      )
    );
    offset += part.length;
  }
  return nodes;
}
