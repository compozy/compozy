"use client";

import * as React from "react";

/** Lines the plain first paint shows: enough to fill a tall pane, cheap for a 1 MB file. */
const PLAIN_WINDOW = 120;
/** Lines kept above a target, so the plain paint sits near where the highlight scrolls. */
const PLAIN_LEAD = 20;

/** `count` lines of `text` from 1-based `first`, without splitting the whole file. */
function sliceLines(text: string, first: number, count: number): string[] {
  let start = 0;
  for (let line = 1; line < first; line += 1) {
    const newline = text.indexOf("\n", start);
    if (newline < 0) return [];
    start = newline + 1;
  }
  const lines: string[] = [];
  while (lines.length < count && start <= text.length) {
    const newline = text.indexOf("\n", start);
    lines.push(text.slice(start, newline < 0 ? text.length : newline));
    if (newline < 0) break;
    start = newline + 1;
  }
  return lines;
}

export interface CodeViewerPlainTextProps {
  text: string;
  lineCount: number;
  /** Clamped target line; the window starts a little above it. */
  targetLine: number | null;
}

/**
 * The viewer's first paint while the highlighter loads: a window of lines as plain text with
 * line numbers, the target line marked. It is replaced by the highlighted view on its first render.
 */
function CodeViewerPlainText({ text, lineCount, targetLine }: CodeViewerPlainTextProps) {
  const first =
    targetLine === null
      ? 1
      : Math.max(1, Math.min(targetLine - PLAIN_LEAD, lineCount - PLAIN_WINDOW + 1));
  const lines = React.useMemo(() => sliceLines(text, first, PLAIN_WINDOW), [text, first]);
  return (
    <div
      data-slot="code-viewer-plain"
      aria-busy="true"
      className="absolute inset-0 overflow-hidden bg-canvas py-1 font-mono text-code-block leading-5 text-fg-2"
    >
      {lines.map((line, index) => {
        const number = first + index;
        return (
          <div
            key={number}
            data-line={number}
            data-target-line={number === targetLine || undefined}
            className="flex whitespace-pre data-target-line:bg-brand-tint-strong"
          >
            <span className="w-12 shrink-0 pr-3 text-right text-faint tabular-nums select-none">
              {number}
            </span>
            <span className="min-w-0">{line}</span>
          </div>
        );
      })}
    </div>
  );
}

export { CodeViewerPlainText };
