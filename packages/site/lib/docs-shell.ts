/**
 * The docs shell's view of the page tree: sidebar nodes, the reading order prev/next follows, and
 * each page's place (section, folder trail). Built once from `docsSource` and shared by every
 * statically generated page.
 */
import type { Node, Root } from "fumadocs-core/page-tree";
import type { ReactNode } from "react";

import {
  docsSections,
  firstPageUrl,
  normalizeDocsUrl,
  sectionHeads,
  type DocsSection,
} from "@/lib/docs-navigation";
import { docsSource } from "@/lib/source";

/** A sidebar row: a page, a folder (with its landing page as `url`), or a labeled separator. */
export interface DocsNavNode {
  key: string;
  name: string;
  url?: string;
  icon?: ReactNode;
  separator?: boolean;
  children?: DocsNavNode[];
}

/** What a section tab needs on the client: no page tree, just where it goes and what it owns. */
export interface DocsTab {
  id: string;
  label: string;
  count: number;
  href: string;
  /** Top-level path segments under `/docs/` that belong to this section. */
  heads: string[];
}

export interface DocsShellSection extends DocsTab {
  nodes: DocsNavNode[];
}

export interface DocsPlace {
  url: string;
  title: string;
  description: string;
  section: DocsShellSection | null;
  /** Folders from the section down to the page's parent. */
  trail: DocsNavNode[];
  node: DocsNavNode;
}

export interface DocsShell {
  total: number;
  sections: DocsShellSection[];
  tabs: DocsTab[];
  /** Every page in reading order, the docs home first. */
  pages: DocsPlace[];
  byUrl: Map<string, DocsPlace>;
}

type PageInfo = { title: string; description: string } | undefined;

function nameOf(name: ReactNode): string {
  return typeof name === "string" || typeof name === "number" ? String(name) : "";
}

function toNavNodes(nodes: readonly Node[]): DocsNavNode[] {
  const out: DocsNavNode[] = [];
  nodes.forEach((node, order) => {
    if (node.type === "separator") {
      const name = nameOf(node.name);
      out.push({ key: `sep:${order}:${name}`, name, separator: true });
      return;
    }
    if (node.type === "page") {
      const url = normalizeDocsUrl(node.url);
      // Only rows that have an icon carry one: the layout ships every section's nodes to the client.
      out.push({
        key: url,
        name: nameOf(node.name),
        url,
        ...(node.icon ? { icon: node.icon } : {}),
      });
      return;
    }
    const indexUrl = node.index ? normalizeDocsUrl(node.index.url) : undefined;
    // The overview transformer repeats the landing page as the folder's first child: the folder
    // row itself opens it, as in the approved draft, so the duplicate row is dropped.
    const children = node.children.filter(
      child => !(child.type === "page" && normalizeDocsUrl(child.url) === indexUrl)
    );
    out.push({
      key: indexUrl ?? `folder:${node.$id ?? nameOf(node.name)}`,
      name: nameOf(node.name),
      url: indexUrl,
      icon: node.icon,
      children: toNavNodes(children),
    });
  });
  return out;
}

function collect(
  nodes: readonly DocsNavNode[],
  section: DocsShellSection | null,
  trail: DocsNavNode[],
  info: (url: string) => PageInfo,
  into: DocsPlace[]
) {
  for (const node of nodes) {
    if (node.separator) continue;
    if (node.url !== undefined) {
      const page = info(node.url);
      into.push({
        url: node.url,
        title: page?.title ?? node.name,
        description: page?.description ?? "",
        section,
        trail,
        node,
      });
    }
    if (node.children) collect(node.children, section, [...trail, node], info, into);
  }
}

export function buildDocsShell(tree: Root, info: (url: string) => PageInfo): DocsShell {
  const sections: DocsShellSection[] = docsSections(tree).map((section: DocsSection) => ({
    id: section.id,
    label: section.label,
    count: section.count,
    href: normalizeDocsUrl(firstPageUrl(section.items) ?? "/docs"),
    heads: sectionHeads(section),
    nodes: toNavNodes(section.items),
  }));
  const firstSeparator = tree.children.findIndex(node => node.type === "separator");
  const intro = toNavNodes(tree.children.slice(0, firstSeparator < 0 ? undefined : firstSeparator));
  const home = info("/docs");
  const pages: DocsPlace[] = [];
  const homeNode: DocsNavNode = { key: "/docs", name: home?.title ?? "Docs", url: "/docs" };
  pages.push({
    url: "/docs",
    title: home?.title ?? "Docs",
    description: home?.description ?? "",
    section: null,
    trail: [],
    node: homeNode,
  });
  collect(
    intro.filter(node => node.url !== "/docs"),
    null,
    [],
    info,
    pages
  );
  for (const section of sections) collect(section.nodes, section, [], info, pages);
  const tabs = sections.map(({ nodes: _nodes, ...tab }) => tab);
  return {
    total: sections.reduce((sum, section) => sum + section.count, 0) + intro.length,
    sections,
    tabs,
    pages,
    byUrl: new Map(pages.map(page => [page.url, page])),
  };
}

let shell: DocsShell | null = null;

/** The shell model of the real docs, built on first use. */
export function getDocsShell(): DocsShell {
  shell ??= buildDocsShell(docsSource.pageTree, url => {
    const slug = url
      .replace(/^\/docs\/?/, "")
      .split("/")
      .filter(Boolean);
    const page = docsSource.getPage(slug);
    return page ? { title: page.data.title, description: page.data.description ?? "" } : undefined;
  });
  return shell;
}

/** Distinct pages under a sidebar node, its own landing page included. */
export function countNavPages(node: DocsNavNode): number {
  let total = node.url === undefined ? 0 : 1;
  for (const child of node.children ?? []) if (!child.separator) total += countNavPages(child);
  return total;
}

/** The top-level folder a page lives in ("Sessions", "CLI Reference"), when it has one. */
export function topFolderOf(place: DocsPlace): DocsNavNode | null {
  return place.trail[0] ?? (place.node.children ? place.node : null);
}

export function neighboursOf(shell: DocsShell, url: string) {
  const order = shell.pages.findIndex(page => page.url === normalizeDocsUrl(url));
  if (order < 0) return { previous: null, next: null };
  return { previous: shell.pages[order - 1] ?? null, next: shell.pages[order + 1] ?? null };
}
