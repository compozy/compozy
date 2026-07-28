export interface QueuedPrompt {
  id: string;
  text: string;
}

// Collapse a queued prompt to a single-line preview: first non-empty line with
// leading markdown markers stripped, so a heading / list / quote still reads as
// one composer row.
export function queuedPromptPreview(text: string): string {
  const firstLine =
    text
      .split(/\r?\n/)
      .map(line => line.trim())
      .find(line => line.length > 0) ?? "";
  if (firstLine.length === 0) {
    return "Queued prompt";
  }
  if (/^(?:`{3,}|~{3,})/.test(firstLine)) {
    return "Code block";
  }
  const normalized = firstLine
    .replace(/^#{1,6}\s+/, "")
    .replace(/^>\s?/, "")
    .replace(/^- \[[ xX]\]\s+/, "")
    .replace(/^[-*+]\s+/, "")
    .replace(/^\d+[.)]\s+/, "")
    .trim();
  return normalized.length > 0 ? normalized : "Queued prompt";
}
