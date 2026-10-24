import { useEffect } from "react";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { $isTextNode, TextNode, type LexicalNode } from "lexical";

const LINK_STYLE = "color: var(--color-info); --compozy-composer-link: 1;";
const URL_PATTERN = /(?:https?:\/\/|mailto:)[^\s<>"`]+/giu;

interface TextSegment {
  text: string;
  link: boolean;
}

function linkRanges(text: string): { start: number; end: number }[] {
  const ranges: { start: number; end: number }[] = [];
  for (const match of text.matchAll(URL_PATTERN)) {
    if (match.index > 0 && /[\w@]/u.test(text[match.index - 1]!)) continue;
    let token = match[0].replace(/[.,;:!?]+$/u, "");
    while (token.endsWith(")") && token.split(")").length > token.split("(").length) {
      token = token.slice(0, -1);
    }
    try {
      const url = new URL(token);
      if (url.protocol === "mailto:" ? !url.pathname : !url.hostname) continue;
      ranges.push({ start: match.index, end: match.index + token.length });
    } catch {
      continue;
    }
  }
  return ranges;
}

/** The text split into alternating plain and link segments, in order. */
function linkSegments(text: string): TextSegment[] {
  const segments: TextSegment[] = [];
  let cursor = 0;
  for (const range of linkRanges(text)) {
    if (range.start > cursor) segments.push({ text: text.slice(cursor, range.start), link: false });
    segments.push({ text: text.slice(range.start, range.end), link: true });
    cursor = range.end;
  }
  if (cursor < text.length) segments.push({ text: text.slice(cursor), link: false });
  return segments;
}

function isDecoratable(node: LexicalNode | null): node is TextNode {
  if (!$isTextNode(node) || !node.isSimpleText()) return false;
  const style = node.getStyle();
  return style === "" || style === LINK_STYLE;
}

/** The adjacent plain text nodes around `node`: one line of prose between chips and breaks. */
function textRun(node: TextNode): TextNode[] {
  const run = [node];
  for (
    let prev = node.getPreviousSibling();
    isDecoratable(prev);
    prev = prev.getPreviousSibling()
  ) {
    run.unshift(prev);
  }
  for (let next = node.getNextSibling(); isDecoratable(next); next = next.getNextSibling()) {
    run.push(next);
  }
  return run;
}

function decorated(run: readonly TextNode[], segments: readonly TextSegment[]): boolean {
  return (
    run.length === segments.length &&
    run.every(
      (node, index) =>
        node.getTextContent() === segments[index]!.text &&
        (node.getStyle() === LINK_STYLE) === segments[index]!.link
    )
  );
}

/**
 * The plain-text composer cannot serialize LinkNode children. Decorate ordinary
 * text nodes instead, preserving selection, editing, undo, and the exact wire text.
 * A URL typed one character at a time first matches as a short prefix, so every
 * pass re-reads the whole run of adjacent text nodes, not the edited node alone.
 */
export function SessionComposerLinkPlugin() {
  const [editor] = useLexicalComposerContext();
  useEffect(
    () =>
      editor.registerNodeTransform(TextNode, node => {
        if (!isDecoratable(node)) return;
        const run = textRun(node);
        const segments = linkSegments(run.map(part => part.getTextContent()).join(""));
        if (segments.length === 0 || decorated(run, segments)) return;
        let merged = run[0]!;
        for (const part of run.slice(1)) merged = merged.mergeWithSibling(part.getLatest());
        if (merged.getStyle() !== "") merged.setStyle("");
        const offsets: number[] = [];
        let offset = 0;
        for (const segment of segments.slice(0, -1)) {
          offset += segment.text.length;
          offsets.push(offset);
        }
        const parts = offsets.length > 0 ? merged.splitText(...offsets) : [merged];
        parts.forEach((part, index) => {
          if (segments[index]?.link) part.setStyle(LINK_STYLE);
        });
      }),
    [editor]
  );
  return null;
}
