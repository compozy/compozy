import type * as React from "react";

/** One inline slot: the node it renders as, and the literal text it stands for. */
export interface StreamMarkdownSlot {
  /** Rendered where the slot sits in prose (paragraphs, emphasis, lists, headings). */
  node: React.ReactNode;
  /** Rendered verbatim where markdown cannot hold a node: code, link labels, raw HTML. */
  text: string;
}

/** Inline slots a markdown source places with `composeMarkdownSlots`, by slot id. */
export type StreamMarkdownSlots = Readonly<Record<string, StreamMarkdownSlot>>;

/** Where one slot replaces a range `[start, end)` of the authored text. */
export interface MarkdownSlotPlacement {
  id: string;
  start: number;
  end: number;
}

// A slot is a Private Use Area sentinel: plain text to the markdown parser, so
// the markdown around it forms as written and no authored construct (a link,
// a code span, raw HTML) can be mistaken for one. The literal form marks a
// sentinel that sits where only text may render.
const SLOT_OPEN = "\uE000";
const SLOT_LITERAL_OPEN = "\uE002";
const SLOT_CLOSE = "\uE001";
const RESERVED = /[\uE000-\uE002]/g;
const SENTINEL = /[\uE000\uE002]([^\uE000-\uE002]*)\uE001/g;
const SENTINEL_CHAR = /[\uE000\uE002]/;

/** Element the slot rehype plugin emits; `<StreamMarkdown>` renders it from `slots`. */
export const MARKDOWN_SLOT_TAG = "compozy-markdown-slot";

/**
 * Markdown source for `text` with each placement's range replaced by its slot.
 * The authored text cannot carry a slot of its own: the reserved sentinel
 * characters it may contain become U+FFFD. Placements are ordered and disjoint.
 */
export function composeMarkdownSlots(
  text: string,
  placements: readonly MarkdownSlotPlacement[]
): string {
  let source = "";
  let cursor = 0;
  for (const placement of placements) {
    source += authored(text.slice(cursor, placement.start));
    source += SLOT_OPEN + placement.id.replace(RESERVED, "") + SLOT_CLOSE;
    cursor = placement.end;
  }
  return source + authored(text.slice(cursor));
}

function authored(text: string): string {
  return text.replace(RESERVED, "\uFFFD");
}

/** Restores the literal text of every slot that sits in code. */
export function restoreSlotText(code: string, slots: StreamMarkdownSlots): string {
  if (!SENTINEL_CHAR.test(code)) return code;
  return code.replace(SENTINEL, (match, id: string) =>
    Object.hasOwn(slots, id) ? (slots[id]?.text ?? "") : match
  );
}

interface TreeNode {
  type: string;
  value?: string;
  tagName?: string;
  properties?: Record<string, unknown>;
  children?: TreeNode[];
}

const LITERAL_MDAST_PARENTS = new Set(["link", "linkReference"]);

function markLiteral(value: string): string {
  return value.replaceAll(SLOT_OPEN, SLOT_LITERAL_OPEN);
}

function visitLiteralContexts(node: TreeNode, literal: boolean) {
  if (
    typeof node.value === "string" &&
    (node.type === "html" || (literal && node.type === "text"))
  ) {
    node.value = markLiteral(node.value);
  }
  const inLiteral = literal || LITERAL_MDAST_PARENTS.has(node.type);
  for (const child of node.children ?? []) visitLiteralContexts(child, inLiteral);
}

/**
 * Remark plugin: a slot inside a link label or raw HTML can hold only text, so
 * it is marked to render as its literal text there.
 */
export function remarkMarkdownSlotLiterals() {
  return (tree: TreeNode) => visitLiteralContexts(tree, false);
}

function splitSlots(value: string): TreeNode[] {
  const nodes: TreeNode[] = [];
  let cursor = 0;
  for (const match of value.matchAll(SENTINEL)) {
    if (match.index > cursor) nodes.push({ type: "text", value: value.slice(cursor, match.index) });
    nodes.push({
      type: "element",
      tagName: MARKDOWN_SLOT_TAG,
      properties: {
        dataMarkdownSlot: match[1],
        dataMarkdownSlotLiteral: match[0].startsWith(SLOT_LITERAL_OPEN) ? "true" : undefined,
      },
      children: [],
    });
    cursor = match.index + match[0].length;
  }
  if (cursor < value.length) nodes.push({ type: "text", value: value.slice(cursor) });
  return nodes;
}

function visitSlots(node: TreeNode) {
  // Code keeps its text; the code renderer restores the literal there.
  if (node.type === "element" && node.tagName === "code") return;
  if (!node.children) return;
  node.children = node.children.flatMap(child => {
    if (
      child.type === "text" &&
      typeof child.value === "string" &&
      SENTINEL_CHAR.test(child.value)
    ) {
      return splitSlots(child.value);
    }
    visitSlots(child);
    return [child];
  });
}

/** Rehype plugin: turns each slot sentinel outside code into a slot element. */
export function rehypeMarkdownSlots() {
  return (tree: TreeNode) => visitSlots(tree);
}
