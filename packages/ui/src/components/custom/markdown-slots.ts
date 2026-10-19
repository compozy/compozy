import type * as React from "react";

/** One inline slot: the node it renders as, and the literal text it stands for. */
export interface StreamMarkdownSlot {
  /** Rendered where the slot sits in prose (paragraphs, emphasis, lists, headings). */
  node: React.ReactNode;
  /** Rendered verbatim where markdown cannot hold a node: inline, fenced, or indented code. */
  text: string;
}

/** Inline slots a markdown source places with `markdownSlotSource`, by slot id. */
export type StreamMarkdownSlots = Readonly<Record<string, StreamMarkdownSlot>>;

/**
 * Markdown source that places the inline slot `id`: a link to `#id`, which
 * `<StreamMarkdown slots>` renders as that slot's node. Because the slot is
 * part of the parsed document, the markdown around it (emphasis, lists,
 * headings) still forms as written. Inside code the slot reads as its `text`.
 * `id` must be a plain identifier.
 */
export function markdownSlotSource(id: string): string {
  return `[${id}](#${id})`;
}

/** Restores the literal text of every slot placed in a code span or block. */
export function restoreSlotText(code: string, slots: StreamMarkdownSlots): string {
  if (!code.includes("](#")) return code;
  let restored = code;
  for (const [id, slot] of Object.entries(slots)) {
    restored = restored.replaceAll(markdownSlotSource(id), slot.text);
  }
  return restored;
}
