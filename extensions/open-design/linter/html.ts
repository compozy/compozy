import { SLOP_EMOJI } from "./rules";

const VOID_TAGS = new Set([
  "area",
  "base",
  "br",
  "col",
  "embed",
  "hr",
  "img",
  "input",
  "link",
  "meta",
  "param",
  "source",
  "track",
  "wbr",
]);

export function findStructuralEmoji(html: string): { emoji: string; text: string } | undefined {
  const content = html.replace(/<(script|style)\b[^>]*>[\s\S]*?<\/\1\s*>/gi, "");
  const stack: { tag: string; structural: boolean }[] = [];
  const tokens = /<(\/?)([a-z][\w:-]*)\b(?:[^>"']|"[^"]*"|'[^']*')*>|([^<]+)/gi;
  for (const token of content.matchAll(tokens)) {
    const text = token[3];
    if (text !== undefined) {
      if (!stack.at(-1)?.structural) continue;
      const emoji = SLOP_EMOJI.find(value => text.includes(value));
      if (emoji) return { emoji, text };
      continue;
    }
    const tag = token[2]?.toLowerCase();
    if (!tag) continue;
    if (token[1]) {
      const index = stack.findLastIndex(element => element.tag === tag);
      if (index >= 0) stack.length = index;
      continue;
    }
    if (VOID_TAGS.has(tag)) continue;
    const structural =
      stack.at(-1)?.structural ||
      /^(?:h[1-6]|button|li)$/.test(tag) ||
      (tag === "span" && hasIconClass(token[0]));
    stack.push({ tag, structural });
  }
  return undefined;
}

function hasIconClass(startTag: string): boolean {
  const attributes = /([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?/g;
  for (const attribute of startTag.matchAll(attributes)) {
    if (attribute[1]?.toLowerCase() !== "class") continue;
    return (attribute[2] ?? attribute[3] ?? attribute[4] ?? "")
      .split(/\s+/)
      .some(value => value.includes("icon"));
  }
  return false;
}
