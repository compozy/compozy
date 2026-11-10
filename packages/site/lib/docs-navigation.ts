import type { Node, Root } from "fumadocs-core/page-tree";
import rootMeta from "@/content/docs/meta.json";

type ApiSection = { label: string; ids: string[] };

/** Section grouping consumed by `scripts/generate-openapi.ts` when it writes the api meta.json. */
export const API_SECTIONS: ApiSection[] = [
  {
    label: "Workspace",
    ids: ["sessions", "workspaces", "worktrees", "agents", "profiles", "skills"],
  },
  {
    label: "Tools & automation",
    ids: [
      "marketplace",
      "tools",
      "toolsets",
      "resources",
      "bundles",
      "automation",
      "loops",
      "notifications",
    ],
  },
  { label: "Observation", ids: ["observe", "hooks"] },
  {
    label: "Operations",
    ids: [
      "daemon",
      "diagnostics",
      "onboarding",
      "filesystem",
      "logs",
      "settings",
      "support",
      "providers",
      "extensions",
      "gateway",
      "app-tokens",
      "vault",
      "agent",
      "tasks",
      "openai",
      "roles",
      "cmd-palette",
    ],
  },
];

function isSeparatorId(id: string): boolean {
  return id.startsWith("---") && id.endsWith("---");
}

function separatorLabel(id: string): string {
  return id.slice(3, -3);
}

/** Fallback group for the ungrouped ids ahead of the first separator (`index`, `how-to-use-…`). */
export const DOCS_ROOT_GROUP = "Docs";

function buildGroupByFolder(): ReadonlyMap<string, string> {
  const groups = new Map<string, string>();
  let current = DOCS_ROOT_GROUP;
  for (const id of rootMeta.pages) {
    if (isSeparatorId(id)) {
      current = separatorLabel(id);
      continue;
    }
    groups.set(id, current);
  }
  return groups;
}

/**
 * D14 sidebar group of each top-level folder/page id, derived from the root meta.json so search
 * grouping can never drift from the sidebar.
 */
export const DOCS_GROUP_BY_FOLDER: ReadonlyMap<string, string> = buildGroupByFolder();

/** Resolves the D14 group for a built docs URL (`/docs/sessions/lifecycle` → `Core concepts`). */
export function docsGroupForUrl(url: string): string {
  const [head] = url.replace(/^\/docs\/?/, "").split("/");
  if (!head) return DOCS_ROOT_GROUP;
  return DOCS_GROUP_BY_FOLDER.get(head) ?? DOCS_ROOT_GROUP;
}

/**
 * Every non-separator id in the root `content/docs/meta.json` must resolve to a top-level folder or
 * page of the built tree. The retired code-synthesized navigation dropped unresolved folders
 * silently (`if (!coreFolder) return pageTree`); this assertion turns that failure mode into a
 * build error.
 */
export function assertDocsTreeComplete(tree: Root): void {
  const resolved = new Set<string>();
  for (const node of tree.children) {
    if (node.type === "folder" && node.$id) {
      resolved.add(node.$id);
      continue;
    }
    if (node.type === "page") {
      const path = node.url.replace(/^\/docs\/?/, "").replace(/\/$/, "");
      resolved.add(path === "" ? "index" : path);
    }
  }

  const missing = rootMeta.pages.filter(id => !isSeparatorId(id) && !resolved.has(id));
  if (missing.length > 0) {
    throw new Error(
      `content/docs/meta.json ids missing from the built page tree: ${missing.join(", ")}. ` +
        "Every listed id must be a top-level folder or page under content/docs."
    );
  }
}

/** One section tab: the pages between two labeled separators of the root `meta.json`. */
export interface DocsSection {
  id: string;
  label: string;
  /** Distinct pages in the section, a folder's landing page counted once. */
  count: number;
  items: Node[];
}

/** `/docs/sessions/lifecycle/` and `/docs/sessions/lifecycle` name the same page. */
export function normalizeDocsUrl(url: string): string {
  const path = url.split(/[?#]/, 1)[0] ?? "";
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}

function plainName(name: unknown): string {
  return typeof name === "string" || typeof name === "number" ? String(name) : "";
}

function sectionIdOf(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

/** Every page URL under `nodes`, the folder landing pages included. */
export function collectPageUrls(nodes: readonly Node[], into = new Set<string>()): Set<string> {
  for (const node of nodes) {
    if (node.type === "page") {
      into.add(normalizeDocsUrl(node.url));
    } else if (node.type === "folder") {
      if (node.index) into.add(normalizeDocsUrl(node.index.url));
      collectPageUrls(node.children, into);
    }
  }
  return into;
}

const SECTIONS = new WeakMap<Root, DocsSection[]>();
const SECTION_BY_URL = new WeakMap<Root, Map<string, DocsSection>>();

/**
 * The section tabs: the root page tree split on its labeled separators, in sidebar order. The pages
 * ahead of the first separator (the docs home and "How to use these docs") belong to no section.
 * Computed once per tree, so 777 statically generated pages share one pass.
 */
export function docsSections(tree: Root): DocsSection[] {
  const cached = SECTIONS.get(tree);
  if (cached) return cached;
  const sections: DocsSection[] = [];
  let current: DocsSection | null = null;
  for (const node of tree.children) {
    if (node.type === "separator") {
      const label = plainName(node.name);
      current = { id: sectionIdOf(label), label, count: 0, items: [] };
      sections.push(current);
      continue;
    }
    current?.items.push(node);
  }
  for (const section of sections) section.count = collectPageUrls(section.items).size;
  SECTIONS.set(tree, sections);
  return sections;
}

/** The section a page belongs to, or `null` for pages outside every section. */
export function sectionOf(tree: Root, url: string): DocsSection | null {
  let byUrl = SECTION_BY_URL.get(tree);
  if (!byUrl) {
    byUrl = new Map();
    for (const section of docsSections(tree)) {
      for (const pageUrl of collectPageUrls(section.items)) byUrl.set(pageUrl, section);
    }
    SECTION_BY_URL.set(tree, byUrl);
  }
  return byUrl.get(normalizeDocsUrl(url)) ?? null;
}

/** Where a section tab lands: the first page of its first folder. */
export function firstPageUrl(nodes: readonly Node[]): string | null {
  for (const node of nodes) {
    if (node.type === "page") return node.url;
    if (node.type !== "folder") continue;
    if (node.index) return node.index.url;
    const nested = firstPageUrl(node.children);
    if (nested !== null) return nested;
  }
  return null;
}

/** The top-level path segments (`sessions`, `cli`) each section owns, for matching a pathname. */
export function sectionHeads(section: DocsSection): string[] {
  const heads = new Set<string>();
  for (const url of collectPageUrls(section.items)) {
    const head = url.replace(/^\/docs\/?/, "").split("/")[0];
    if (head) heads.add(head);
  }
  return [...heads];
}

/** The section whose top-level folders contain `pathname`; `null` outside every section. */
export function currentSectionId(
  sections: readonly { id: string; heads: readonly string[] }[],
  pathname: string
): string | null {
  const head = pathname.replace(/^\/docs\/?/, "").split("/")[0];
  if (!head) return null;
  return sections.find(section => section.heads.includes(head))?.id ?? null;
}
