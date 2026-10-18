import { getBreadcrumbItems } from "fumadocs-core/breadcrumb";
import { findParent, flattenTree, type Root } from "fumadocs-core/page-tree";
import type { ReactNode } from "react";

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

export function resolveProductLabel(_slug: string[]): string {
  return "CompozyOS";
}

export function resolveAudience(_slug: string[]): string {
  return "people running agent work";
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

/**
 * The docs root titles itself `CompozyOS documentation`, which would repeat the product word
 * already standing to its left (`CompozyOS › CompozyOS documentation`). The trail names the
 * destination instead.
 */
const DOCS_ROOT_CRUMB = "Docs";

export function buildMastheadCrumbs(
  tree: Root,
  pageUrl: string,
  pageTitle: string,
  isRoot = false
): DocMastheadCrumb[] {
  if (isRoot) {
    return [{ name: DOCS_ROOT_CRUMB }];
  }

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
  slug: string[],
  tree: Root,
  pageUrl: string,
  pageTitle: string
): DocMastheadMeta {
  return {
    product: resolveProductLabel(slug),
    audience: resolveAudience(slug),
    crumbs: buildMastheadCrumbs(tree, pageUrl, pageTitle, slug.length === 0),
    sectionPageCount: sectionPageCount(tree, pageUrl),
  };
}
