import { readdirSync, statSync } from "node:fs";
import { relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { MARKETING_NAV_LINKS, isCurrentSection } from "../site-nav";
import { contentRoot } from "../content-test-utils";

type InternalLink = {
  label: string;
  href: string;
  source: string;
};

const appRoutes = new Set([
  "/",
  "/blog",
  "/changelog",
  "/blog/feed.xml",
  "/marketplace",
  "/download",
]);

function normalizeRoute(route: string): string {
  return route.replace(/\/$/, "") || "/";
}

function listMdxFiles(dir: string): string[] {
  const files: string[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      files.push(...listMdxFiles(fullPath));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(".mdx")) {
      files.push(fullPath);
    }
  }
  return files.sort();
}

function routeFromContentFile(file: string): string {
  const routeParts = relative(contentRoot, file)
    .replace(/\.mdx$/, "")
    .split("/");
  if (routeParts.at(-1) === "index") {
    routeParts.pop();
  }
  return normalizeRoute(`/${routeParts.join("/")}`);
}

function knownRoutes(): Set<string> {
  return new Set([...appRoutes, ...listMdxFiles(contentRoot).map(routeFromContentFile)]);
}

function marketingLinks(): InternalLink[] {
  return MARKETING_NAV_LINKS.map(link => ({ ...link, source: "MARKETING_NAV_LINKS" }));
}

function expectRouteBackedLinks(links: InternalLink[]) {
  const routes = knownRoutes();
  const missing = links
    .filter(link => !routes.has(normalizeRoute(link.href)))
    .map(link => `${link.source}: ${link.label} -> ${link.href}`);

  expect(missing).toEqual([]);
}

describe("site navigation configuration", () => {
  it("keeps configured internal navigation links backed by real routes", () => {
    expectRouteBackedLinks(marketingLinks());
  });

  it("marks a marketing section current on its route and every route nested under it", () => {
    expect(isCurrentSection("/blog", "/blog")).toBe(true);
    expect(isCurrentSection("/blog/introducing-compozyos", "/blog")).toBe(true);
    expect(isCurrentSection("/blogroll", "/blog")).toBe(false);
    expect(isCurrentSection("/", "/docs")).toBe(false);
  });
});
