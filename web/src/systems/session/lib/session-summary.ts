/** Display-only summaries. Callers retain the original for detail, copy, and find. */
const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });

/** Collapses display whitespace and truncates at grapheme boundaries, preserving the source. */
export function compactSessionSummary(text: string, limit = 80): string {
  const line = text.replace(/\s+/gu, " ").trim();
  const result: string[] = [];
  for (const { segment } of graphemes.segment(line)) {
    if (result.length === limit) return result.slice(0, limit - 1).join("") + "…";
    result.push(segment);
  }
  return result.join("");
}
