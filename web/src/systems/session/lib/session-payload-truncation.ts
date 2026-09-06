// Giant payload truncation (US-019.EC-2): the frame never freezes for a tool
// result. The expanded body shows the head of the output; a footer strip says
// how much is shown of how much and offers Show all (renders the rest in the
// scrolling body) and Download (the payload as a file). Pure model — the note
// is derived from the text itself so it is truthful for inline payloads; a
// daemon-truncated result keeps its own artifact affordance.

/** Lines shown before the strip takes over. */
export const PAYLOAD_PREVIEW_MAX_LINES = 200;

export interface PayloadTruncation {
  /** The text to render: the head while truncated, everything once expanded. */
  text: string;
  truncated: boolean;
  shownLines: number;
  totalLines: number;
  bytes: number;
}

const BYTE_ENCODER = typeof TextEncoder === "undefined" ? null : new TextEncoder();
const LINE_FORMATTER = new Intl.NumberFormat("en-US");

export function payloadByteSize(text: string): number {
  return BYTE_ENCODER ? BYTE_ENCODER.encode(text).length : text.length;
}

export function countLines(text: string): number {
  return text.length === 0 ? 0 : text.split("\n").length;
}

export function truncatePayload(
  text: string,
  options: { maxLines?: number; expanded?: boolean } = {}
): PayloadTruncation {
  const maxLines = Math.max(1, options.maxLines ?? PAYLOAD_PREVIEW_MAX_LINES);
  const totalLines = countLines(text);
  const bytes = payloadByteSize(text);
  if (options.expanded || totalLines <= maxLines) {
    return { text, truncated: false, shownLines: totalLines, totalLines, bytes };
  }
  const head = text.split("\n").slice(0, maxLines).join("\n");
  return { text: head, truncated: true, shownLines: maxLines, totalLines, bytes };
}

/** "1.8 MB", "412 KB", "96 B" — one decimal only above a megabyte. */
export function formatPayloadSize(bytes: number): string {
  if (bytes < 1_024) return `${bytes} B`;
  if (bytes < 1_024 * 1_024) return `${Math.round(bytes / 1_024)} KB`;
  return `${(bytes / (1_024 * 1_024)).toFixed(1)} MB`;
}

/** The strip sentence: "Showing the first 200 of 12,480 lines". */
export function payloadTruncationNote(model: Pick<PayloadTruncation, "shownLines" | "totalLines">) {
  return `Showing the first ${LINE_FORMATTER.format(model.shownLines)} of ${LINE_FORMATTER.format(model.totalLines)} lines`;
}

/** Whether a payload deserves the strip at all: it overflows the preview. */
export function payloadOverflows(text: string, maxLines = PAYLOAD_PREVIEW_MAX_LINES): boolean {
  return countLines(text) > maxLines;
}
