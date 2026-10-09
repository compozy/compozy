// Plain-text reading of one markdown line, for surfaces that show agent text
// as a single unformatted line (queued previews, subagent rows). Markers are
// dropped; the words they wrapped stay.

/** Leading block markers: heading, quote, task box, bullet, ordered number. */
export function stripMarkdownLineMarkers(line: string): string {
  return line
    .replace(/^#{1,6}(?:\s+|$)/, "")
    .replace(/^>\s?/, "")
    .replace(/^- \[[ xX]\](?:\s+|$)/, "")
    .replace(/^[-*+](?:\s+|$)/, "")
    .replace(/^\d+[.)](?:\s+|$)/, "")
    .trim();
}

/** Inline markers: links and images keep their text, emphasis and code spans lose their delimiters. */
export function stripMarkdownInline(text: string): string {
  return text
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/`([^`]*)`/g, "$1")
    .replace(/(\*\*|__)(?=\S)(.+?)(?<=\S)\1/g, "$2")
    .replace(/~~(?=\S)(.+?)(?<=\S)~~/g, "$1")
    .replace(/(^|[^\w*])\*(?=\S)([^*]+?)(?<=\S)\*(?!\w)/g, "$1$2")
    .replace(/(^|[^\w_])_(?=\S)([^_]+?)(?<=\S)_(?!\w)/g, "$1$2");
}

/** One markdown line as plain text. */
export function markdownLineToPlainText(line: string): string {
  return stripMarkdownInline(stripMarkdownLineMarkers(line.trim())).trim();
}
