interface TreeNode {
  type: string;
  url?: string;
  children?: TreeNode[];
}

const SCHEME = /^[a-z][a-z0-9+.-]*:/i;

/**
 * `href` resolved against `base` (a root-relative directory such as `docs/specs`) as a
 * root-anchored path (`/docs/specs/a.md#x`); `""` when it climbs out of the root, which safe
 * mode renders blocked; `null` when it is not a document-relative reference.
 */
export function resolveDocumentRelativeUrl(base: string, href: string): string | null {
  const trimmed = href.trim();
  if (
    trimmed === "" ||
    trimmed.startsWith("#") ||
    trimmed.startsWith("/") ||
    SCHEME.test(trimmed)
  ) {
    return null;
  }
  const suffixAt = trimmed.search(/[?#]/);
  const pathPart = suffixAt < 0 ? trimmed : trimmed.slice(0, suffixAt);
  const suffix = suffixAt < 0 ? "" : trimmed.slice(suffixAt);
  const segments = base.split("/").filter(segment => segment !== "" && segment !== ".");
  for (const segment of pathPart.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === "..") {
      if (segments.length === 0) return "";
      segments.pop();
    } else {
      segments.push(segment);
    }
  }
  return `/${segments.join("/")}${suffix}`;
}

function visit(node: TreeNode, base: string) {
  if ((node.type === "link" || node.type === "image" || node.type === "definition") && node.url) {
    const resolved = resolveDocumentRelativeUrl(base, node.url);
    if (resolved !== null) node.url = resolved;
  }
  for (const child of node.children ?? []) visit(child, base);
}

/**
 * Remark plugin: rewrites document-relative link and image URLs to root-anchored paths before the
 * safe-mode passes, which otherwise resolve them against the site root and lose the document's
 * directory. Options: `{ base }`, the document's root-relative directory.
 */
export function remarkRelativeUrls(options?: { base?: string }) {
  const base = options?.base ?? "";
  return (tree: TreeNode) => {
    visit(tree, base);
  };
}
