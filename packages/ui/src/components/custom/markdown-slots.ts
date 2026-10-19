import type * as React from "react";

/** Inline nodes a markdown source places with `markdownSlotSource`, by slot id. */
export type StreamMarkdownSlots = Readonly<Record<string, React.ReactNode>>;

/**
 * Markdown source that places the inline slot `id`: a link to `#id`, which
 * `<StreamMarkdown slots>` renders as that slot's node. Because the slot is
 * part of the parsed document, the markdown around it (emphasis, lists,
 * headings) still forms as written. `id` must be a plain identifier.
 */
export function markdownSlotSource(id: string): string {
  return `[${id}](#${id})`;
}
