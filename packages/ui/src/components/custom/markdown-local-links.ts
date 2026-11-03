import { defaultRehypePlugins } from "streamdown";

/** Element the local-link rehype plugin emits; `<StreamMarkdown>` renders it with the caller's component. */
export const MARKDOWN_LOCAL_LINK_TAG = "compozy-markdown-local-link";

interface TreeNode {
  type: string;
  tagName?: string;
  url?: string;
  properties?: Record<string, unknown>;
  children?: TreeNode[];
}

interface PipelineFile {
  data: Record<string, unknown>;
}

type Transformer = (tree: TreeNode) => TreeNode | undefined | void;
type Attacher = (options?: unknown) => Transformer;

const ROUTED_KEY = "compozyMarkdownLocalLinks";
const TOKEN_PREFIX = "compozy-local-link-";
const OPENS_EXTERNALLY = new Set(["http", "https", "mailto"]);
// Script-capable or content-embedding schemes never reach the caller's component.
const NEVER_ROUTED = new Set(["javascript", "vbscript", "data", "blob"]);
const SCHEME = /^([a-z][a-z0-9+.-]*):/i;
// The URL parser drops tab and newline anywhere, and C0 controls or spaces at either end.
const URL_IGNORED = /[\t\n\r]/g;

function trimUrlEdges(value: string): string {
  let start = 0;
  let end = value.length;
  while (start < end && value.charCodeAt(start) <= 0x20) start += 1;
  while (end > start && value.charCodeAt(end - 1) <= 0x20) end -= 1;
  return value.slice(start, end);
}

function urlScheme(href: string): string | undefined {
  const normalized = trimUrlEdges(href.replace(URL_IGNORED, ""));
  return SCHEME.exec(normalized)?.[1]?.toLowerCase();
}

function runSafePass(name: string, tree: TreeNode): TreeNode {
  const entry = defaultRehypePlugins[name];
  if (entry === undefined) return tree;
  const [attacher, options] = (Array.isArray(entry) ? entry : [entry]) as [Attacher, unknown?];
  return attacher(options)(tree) ?? tree;
}

/** Whether the safe-mode sanitize and harden passes would render `href` as a blocked link. */
function safeModeBlocks(href: string): boolean {
  let tree: TreeNode = {
    type: "root",
    children: [{ type: "element", tagName: "a", properties: { href }, children: [] }],
  };
  tree = runSafePass("sanitize", tree);
  tree = runSafePass("harden", tree);
  return tree.children?.[0]?.tagName !== "a";
}

function routesToLocalLink(href: string): boolean {
  if (href.trim() === "") return false;
  const scheme = urlScheme(href);
  if (scheme !== undefined && (OPENS_EXTERNALLY.has(scheme) || NEVER_ROUTED.has(scheme))) {
    return false;
  }
  return safeModeBlocks(href);
}

function visitLinks(node: TreeNode, visit: (link: TreeNode & { url: string }) => void) {
  if (node.type === "link" && typeof node.url === "string")
    visit(node as TreeNode & { url: string });
  for (const child of node.children ?? []) visitLinks(child, visit);
}

/**
 * Remark plugin: each link the safe-mode passes would block, other than an
 * http(s), mailto, or script-capable destination, gets an unguessable
 * fragment for this parse so harden keeps it; `rehypeMarkdownLocalLinks`
 * turns it back into its authored destination after the safe passes.
 */
export function remarkMarkdownLocalLinks() {
  return (tree: TreeNode, file: PipelineFile) => {
    const routed = new Map<string, string>();
    const nonce = Math.random().toString(36).slice(2);
    visitLinks(tree, link => {
      if (!routesToLocalLink(link.url)) return;
      const token = `#${TOKEN_PREFIX}${nonce}-${routed.size}`;
      routed.set(token, link.url);
      link.url = token;
    });
    if (routed.size > 0) file.data[ROUTED_KEY] = routed;
  };
}

function visitAnchors(node: TreeNode, routed: ReadonlyMap<string, string>) {
  if (node.type === "element" && node.tagName === "a") {
    const href = node.properties?.href;
    const authored = typeof href === "string" ? routed.get(href) : undefined;
    if (authored !== undefined) {
      node.tagName = MARKDOWN_LOCAL_LINK_TAG;
      node.properties = { dataHref: authored };
    }
  }
  for (const child of node.children ?? []) visitAnchors(child, routed);
}

/** Rehype plugin, appended after the safe passes: emits each routed link as a local-link element. */
export function rehypeMarkdownLocalLinks() {
  return (tree: TreeNode, file: PipelineFile) => {
    const routed = file.data[ROUTED_KEY];
    if (routed instanceof Map) visitAnchors(tree, routed as ReadonlyMap<string, string>);
  };
}
