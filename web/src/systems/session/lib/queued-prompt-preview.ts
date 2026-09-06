/**
 * One-line preview of a queued follow-up (US-005.EC-3). Multi-line text shows
 * its first meaningful line with leading markdown markers stripped; a message
 * that opens with a fence reads as `code` plus the first line inside the fence.
 */
export interface QueuedPromptPreview {
  kind: "text" | "code";
  text: string;
}

const EMPTY_PREVIEW: QueuedPromptPreview = { kind: "text", text: "Queued message" };
const FENCE_PATTERN = /^(?:`{3,}|~{3,})/;

function stripLineMarkers(line: string): string {
  return line
    .replace(/^#{1,6}(?:\s+|$)/, "")
    .replace(/^>\s?/, "")
    .replace(/^- \[[ xX]\](?:\s+|$)/, "")
    .replace(/^[-*+](?:\s+|$)/, "")
    .replace(/^\d+[.)](?:\s+|$)/, "")
    .trim();
}

export function queuedPromptPreview(text: string): QueuedPromptPreview {
  const lines = text.split(/\r?\n/).map(line => line.trim());
  const firstIndex = lines.findIndex(line => line.length > 0);
  if (firstIndex < 0) {
    return EMPTY_PREVIEW;
  }
  const first = lines[firstIndex]!;
  if (FENCE_PATTERN.test(first)) {
    const inside = lines
      .slice(firstIndex + 1)
      .find(line => line.length > 0 && !FENCE_PATTERN.test(line));
    return { kind: "code", text: inside ?? "Code block" };
  }
  const normalized = stripLineMarkers(first);
  return normalized.length > 0 ? { kind: "text", text: normalized } : EMPTY_PREVIEW;
}
