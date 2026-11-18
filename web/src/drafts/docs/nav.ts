/** A real snapshot of the docs navigation tree. Production reads it from the Fumadocs source. */
import raw from "./nav.json";

export interface NavNode {
  /** Sidebar label. */
  t: string;
  /** Page slug under /docs; folders without a landing page have none. */
  s?: string;
  d?: string;
  i?: string;
  m?: string;
  /** The landing page's own title, when it differs from the folder label. */
  h?: string;
  c?: NavNode[];
  sep?: string;
}

export interface NavGroup {
  id: string;
  label: string;
  count: number;
  items: NavNode[];
}

interface NavData {
  home: { t: string; d: string };
  total: number;
  groups: NavGroup[];
}

export interface DocPage {
  slug: string;
  title: string;
  description: string;
  maturity: string | null;
  group: NavGroup | null;
  /** Folders from the group down to the page's parent. */
  trail: NavNode[];
  node: NavNode;
}

const DATA = raw as unknown as NavData;

export const TOTAL_PAGES = DATA.total;
export const GROUPS = DATA.groups.filter(group => group.label !== "");
export const INTRO_PAGES = DATA.groups.find(group => group.label === "")?.items ?? [];

const HOME_PAGE: DocPage = {
  slug: "",
  title: DATA.home.t,
  description: DATA.home.d,
  maturity: null,
  group: null,
  trail: [],
  node: { t: DATA.home.t, s: "", d: DATA.home.d },
};

function collect(
  nodes: readonly NavNode[],
  group: NavGroup | null,
  trail: NavNode[],
  into: DocPage[]
) {
  for (const node of nodes) {
    if (node.sep !== undefined) continue;
    if (node.s !== undefined) {
      into.push({
        slug: node.s,
        title: node.h ?? node.t,
        description: node.d ?? "",
        maturity: node.m ?? null,
        group,
        trail,
        node,
      });
    }
    if (node.c) collect(node.c, group, [...trail, node], into);
  }
}

function buildPages() {
  const pages: DocPage[] = [HOME_PAGE];
  collect(INTRO_PAGES, null, [], pages);
  for (const group of GROUPS) collect(group.items, group, [], pages);
  return pages;
}

/** Every page in reading order: the order prev/next follows. */
export const PAGES = buildPages();

const BY_SLUG = new Map(PAGES.map(page => [page.slug, page]));
const ORDER = new Map(PAGES.map((page, order) => [page.slug, order]));

export function findPage(slug: string) {
  return BY_SLUG.get(slug) ?? HOME_PAGE;
}

export function neighbours(slug: string) {
  const order = ORDER.get(slug) ?? 0;
  return { previous: PAGES[order - 1] ?? null, next: PAGES[order + 1] ?? null };
}

export function countPages(node: NavNode): number {
  let total = node.s === undefined ? 0 : 1;
  for (const child of node.c ?? []) if (child.sep === undefined) total += countPages(child);
  return total;
}

/** The top-level folder a page lives in: "Sessions", "CLI Reference". */
export function sectionOf(page: DocPage) {
  return page.trail[0] ?? (page.node.c ? page.node : null);
}

export function slugFromHref(href: string) {
  if (!href.startsWith("/docs")) return null;
  const slug = href
    .replace(/^\/docs\/?/, "")
    .replace(/[#?].*$/, "")
    .replace(/\/$/, "");
  return BY_SLUG.has(slug) ? slug : null;
}

function score(page: DocPage, needle: string) {
  const title = page.title.toLowerCase();
  if (title === needle) return 100;
  if (title.startsWith(needle)) return 80;
  if (title.includes(needle)) return 60;
  if (page.slug.includes(needle)) return 40;
  if (page.description.toLowerCase().includes(needle)) return 20;
  return 0;
}

export interface SearchHit {
  page: DocPage;
  score: number;
}

export function searchPages(query: string, groupId: string | null): SearchHit[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return [];
  const hits: SearchHit[] = [];
  for (const page of PAGES) {
    if (groupId && page.group?.id !== groupId) continue;
    const rank = score(page, needle);
    if (rank > 0) hits.push({ page, score: rank });
  }
  hits.sort((a, b) => b.score - a.score || a.page.slug.length - b.page.slug.length);
  return hits;
}

/** A CLI page's last word is the command; the rest is its path. */
export function shortLabel(node: NavNode) {
  if (!node.t.startsWith("compozy ")) return node.t;
  return node.t.split(" ").at(-1) ?? node.t;
}

export function containsSlug(node: NavNode, slug: string): boolean {
  if (node.s === slug) return true;
  return (node.c ?? []).some(child => child.sep === undefined && containsSlug(child, slug));
}

function firstIn(nodes: readonly NavNode[]): string | null {
  for (const node of nodes) {
    if (node.sep !== undefined) continue;
    if (node.s !== undefined) return node.s;
    const nested = firstIn(node.c ?? []);
    if (nested !== null) return nested;
  }
  return null;
}

/** Where a section tab lands: the first page of its first folder. */
export function firstSlug(group: NavGroup) {
  return firstIn(group.items) ?? "";
}

export function keyOf(node: NavNode) {
  return node.s ?? node.sep ?? node.t;
}
