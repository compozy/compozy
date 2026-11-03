// S9 Read mode: what the viewer's Markdown shows of a proposed skill file, and whether it leaves
// anything out. The check parses with the renderer's own pipeline (remark-parse + remark-gfm, as
// Streamdown uses), so anything the renderer drops is caught from the tree, not guessed by a regex.
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import { unified } from "unified";

import { parseFrontmatter, splitFrontmatter } from "./agent-frontmatter";

/**
 * Nodes Read renders as nothing: raw HTML and comments (stripped at the parser), link reference
 * definitions (the `[//]: # (…)` comment idiom) and footnote definitions.
 */
const HIDDEN_NODE_TYPES: ReadonlySet<string> = new Set([
  "html",
  "definition",
  "footnoteDefinition",
]);
/** Zero-width and bidi control characters: present in the text, invisible on screen. */
const INVISIBLE_CHARACTERS = /[​-‏‪-‮⁦-⁩﻿]/;
const READ_METADATA_KEYS: ReadonlySet<string> = new Set(["name", "description"]);

const parser = unified().use(remarkParse).use(remarkGfm);

/** The part of an mdast node the walk reads. */
interface MarkdownNode {
  type: string;
  children?: readonly MarkdownNode[];
}

function hasHiddenNode(node: MarkdownNode): boolean {
  if (HIDDEN_NODE_TYPES.has(node.type)) return true;
  return node.children?.some(child => hasHiddenNode(child)) ?? false;
}

/** Whether Read would leave out text this Markdown body contains. */
export function markdownHidesText(body: string): boolean {
  if (INVISIBLE_CHARACTERS.test(body)) return true;
  return hasHiddenNode(parser.runSync(parser.parse(body)) as MarkdownNode);
}

/**
 * What Read shows of a Markdown file: the body without its frontmatter. `hidesText` is true when
 * Read leaves something out besides the name and description, so the viewer points to Source.
 */
export function skillMarkdownRead(body: string): { text: string; hidesText: boolean } {
  const split = splitFrontmatter(body);
  if (!split) return { text: body, hidesText: true };
  const metadata = split.metadata === null ? {} : parseFrontmatter(split.metadata);
  const extraMetadata =
    metadata === null || Object.keys(metadata).some(key => !READ_METADATA_KEYS.has(key));
  return { text: split.body, hidesText: extraMetadata || markdownHidesText(split.body) };
}
