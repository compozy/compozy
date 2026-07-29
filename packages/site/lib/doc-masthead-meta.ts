import { getBreadcrumbItems } from "fumadocs-core/breadcrumb";
import { findParent, flattenTree, type Root } from "fumadocs-core/page-tree";
import type { ReactNode } from "react";

export type DocKind = "runtime" | "protocol";

export interface DocMastheadCrumb {
  name: string;
  href?: string;
}

export interface DocMastheadMeta {
  product: string;
  audience: string;
  crumbs: DocMastheadCrumb[];
  sectionPageCount: number | null;
}

function nodeName(value: ReactNode): string {
  if (typeof value === "string" || typeof value === "number") {
    return String(value);
  }
  return "";
}

export function resolveProductLabel(kind: DocKind, slug: string[]): string {
  if (kind === "runtime") {
    return "CompozyOS Runtime";
  }
  return slug[0] === "specification" ? "Compozy Network Protocol" : "Compozy Network";
}

export function resolveAudience(kind: DocKind): string {
  // COPY.md §4 audience names — never "operators" (control-room drift).
  return kind === "runtime" ? "people running agent work" : "protocol implementers";
}

export function sectionPageCount(tree: Root, pageUrl: string): number | null {
  const parent = findParent(tree, pageUrl);
  if (!parent || !("children" in parent)) {
    return null;
  }
  const nested = flattenTree(parent.children);
  const index = parent.type === "folder" && parent.index ? parent.index : undefined;
  const count = index ? 1 + nested.filter(page => page.url !== index.url).length : nested.length;
  return count > 0 ? count : null;
}

export function buildMastheadCrumbs(
  tree: Root,
  pageUrl: string,
  pageTitle: string
): DocMastheadCrumb[] {
  const items = getBreadcrumbItems(pageUrl, tree, { includePage: true });
  const crumbs: DocMastheadCrumb[] = items.map(item => ({
    name: nodeName(item.name) || pageTitle,
    href: item.url,
  }));

  if (crumbs.length === 0) {
    return [{ name: pageTitle }];
  }

  const last = crumbs[crumbs.length - 1];
  if (last.name !== pageTitle) {
    last.name = pageTitle;
  }
  delete last.href;

  return crumbs;
}

export function resolveDocMastheadMeta(
  kind: DocKind,
  slug: string[],
  tree: Root,
  pageUrl: string,
  pageTitle: string
): DocMastheadMeta {
  return {
    product: resolveProductLabel(kind, slug),
    audience: resolveAudience(kind),
    crumbs: buildMastheadCrumbs(tree, pageUrl, pageTitle),
    sectionPageCount: sectionPageCount(tree, pageUrl),
  };
}
