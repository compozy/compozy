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
/**
 * Characters that are in the text but invisible on screen: soft hyphen, Mongolian vowel separator,
 * zero-width and bidi controls, word joiner and invisible operators, BOM, and Unicode tag characters
 * (the "ASCII smuggling" carrier). Built from an escaped string so no tool can normalize them away.
 */
const INVISIBLE_CHARACTERS = new RegExp(
  "[\\u00AD\\u180E\\u200B-\\u200F\\u202A-\\u202E\\u2060-\\u2064\\u2066-\\u2069\\uFEFF\\u{E0000}-\\u{E007F}]",
  "u"
);

/** Whether any file, Markdown or not, holds characters the screen won't show. */
export function hasInvisibleCharacters(text: string): boolean {
  return INVISIBLE_CHARACTERS.test(text);
}
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
  if (hasInvisibleCharacters(body)) return true;
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
