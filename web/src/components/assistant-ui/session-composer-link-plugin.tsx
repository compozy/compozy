import { useEffect } from "react";
import { useLexicalComposerContext } from "@lexical/react/LexicalComposerContext";
import { TextNode } from "lexical";

const LINK_STYLE = "color: var(--color-info); --compozy-composer-link: 1;";
const URL_PATTERN = /(?:https?:\/\/|mailto:)[^\s<>"`]+/giu;

function firstLink(text: string): { start: number; end: number } | undefined {
  for (const match of text.matchAll(URL_PATTERN)) {
    if (match.index > 0 && /[\w@]/u.test(text[match.index - 1]!)) continue;
    let token = match[0].replace(/[.,;:!?]+$/u, "");
    while (token.endsWith(")") && token.split(")").length > token.split("(").length) {
      token = token.slice(0, -1);
    }
    try {
      const url = new URL(token);
      if (url.protocol === "mailto:" ? !url.pathname : !url.hostname) continue;
      return { start: match.index, end: match.index + token.length };
    } catch {
      continue;
    }
  }
  return undefined;
}

/**
 * The plain-text composer cannot serialize LinkNode children. Decorate ordinary
 * text nodes instead, preserving selection, editing, undo, and the exact wire text.
 */
export function SessionComposerLinkPlugin() {
  const [editor] = useLexicalComposerContext();
  useEffect(
    () =>
      editor.registerNodeTransform(TextNode, node => {
        if (!node.isSimpleText()) return;
        const text = node.getTextContent();
        const link = firstLink(text);
        if (!link) {
          if (node.getStyle() === LINK_STYLE) node.setStyle("");
          return;
        }
        if (link.start === 0 && link.end === text.length) {
          if (node.getStyle() !== LINK_STYLE) node.setStyle(LINK_STYLE);
          return;
        }
        const parts = node.splitText(link.start, link.end);
        for (const part of parts) {
          if (part.getStyle() === LINK_STYLE) part.setStyle("");
        }
        parts[link.start === 0 ? 0 : 1]?.setStyle(LINK_STYLE);
      }),
    [editor]
  );
  return null;
}
