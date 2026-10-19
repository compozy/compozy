import { $getRoot, $isElementNode, type RangeSelection } from "lexical";

/** The composer's flattened text (paragraphs joined by `\n`) and the caret offset in it. */
export interface EditorCursorSnapshot {
  text: string;
  cursor: number;
}

/**
 * Walks every paragraph once to flatten the composer text and place the
 * caret. Must run inside an editor-state read; null when the anchor is not
 * inside a paragraph.
 */
export function readCursorSnapshot(selection: RangeSelection): EditorCursorSnapshot | null {
  const anchor = selection.anchor;
  const anchorNode = anchor.getNode();
  const root = $getRoot();
  let text = "";
  let cursor = -1;
  let paragraphIndex = 0;

  for (const paragraph of root.getChildren()) {
    if (paragraphIndex > 0) text += "\n";
    paragraphIndex += 1;
    if (!$isElementNode(paragraph)) continue;

    if (anchor.type === "element" && anchorNode === paragraph) {
      // Element anchor: offset counts children before the caret.
      const paragraphStart = text.length;
      let cursorInParagraph = 0;
      let childIndex = 0;
      for (const child of paragraph.getChildren()) {
        if (childIndex < anchor.offset) cursorInParagraph += child.getTextContent().length;
        text += child.getTextContent();
        childIndex += 1;
      }
      cursor = paragraphStart + cursorInParagraph;
      continue;
    }

    for (const child of paragraph.getChildren()) {
      if (anchor.type === "text" && child === anchorNode) {
        cursor = text.length + anchor.offset;
      }
      text += child.getTextContent();
    }
  }
  if (cursor < 0) return null;
  return { text, cursor };
}
