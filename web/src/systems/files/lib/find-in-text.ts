import type { CodeViewerMatch } from "@compozy/ui";

/** Matches past this many are not listed; the count reads "10,000+". */
export const FIND_MATCH_LIMIT = 10_000;

const COUNT = new Intl.NumberFormat("en-US");

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/**
 * Every literal occurrence of `query` in `text`, by 1-based line and UTF-16 range within the
 * line: the viewer's own line model, so lines that were never rendered are found too
 * (US-015). Stops at `FIND_MATCH_LIMIT`.
 */
export function findInText(text: string, query: string, matchCase: boolean): CodeViewerMatch[] {
  if (query === "") return [];
  const pattern = new RegExp(escapeRegExp(query), matchCase ? "g" : "gi");
  const matches: CodeViewerMatch[] = [];
  let lineStart = 0;
  let line = 1;
  while (lineStart <= text.length) {
    const newline = text.indexOf("\n", lineStart);
    const lineEnd = newline < 0 ? text.length : newline;
    const lineText = text.slice(lineStart, lineEnd);
    pattern.lastIndex = 0;
    for (let found = pattern.exec(lineText); found; found = pattern.exec(lineText)) {
      matches.push({ line, start: found.index, end: found.index + found[0].length });
      if (matches.length >= FIND_MATCH_LIMIT) return matches;
    }
    if (newline < 0) break;
    lineStart = newline + 1;
    line += 1;
  }
  return matches;
}

/** The first match at or after `line`, else the first match; `null` without matches. */
export function firstMatchFrom(matches: readonly CodeViewerMatch[], line?: number): number | null {
  if (matches.length === 0) return null;
  if (line === undefined) return 0;
  const index = matches.findIndex(match => match.line >= line);
  return index < 0 ? 0 : index;
}

/** "3 of 17", "No results", or nothing before the first keystroke. */
export function findCountLabel(find: {
  query: string;
  matches: readonly CodeViewerMatch[];
  active: number | null;
}): string {
  if (find.query === "") return "";
  if (find.matches.length === 0 || find.active === null) return "No results";
  const total = COUNT.format(find.matches.length);
  const capped = find.matches.length >= FIND_MATCH_LIMIT ? `${total}+` : total;
  return `${COUNT.format(find.active + 1)} of ${capped}`;
}
