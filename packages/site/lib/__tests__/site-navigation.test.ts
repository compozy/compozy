import { readdirSync, statSync } from "node:fs";
import { relative, resolve } from "node:path";
import { describe, expect, it } from "vitest";
import { footerColumns } from "../footer-config";
import { baseOptions } from "../layout.shared";
import { siteConfig } from "../site-config";
import { contentRoot } from "../content-test-utils";

type InternalLink = {
  label: string;
  href: string;
  source: string;
};

const appRoutes = new Set(["/", "/blog", "/changelog", "/blog/feed.xml"]);

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

function layoutLinks(): InternalLink[] {
  const links = Array.isArray(baseOptions.links) ? baseOptions.links : [];
  return links.map((link, index) => {
    const entry = link as { text?: unknown; url?: unknown };
    return {
      label: typeof entry.text === "string" ? entry.text : `link ${index}`,
      href: typeof entry.url === "string" ? entry.url : "",
      source: "baseOptions.links",
    };
  });
}

function footerInternalLinks(): InternalLink[] {
  return footerColumns.flatMap(column =>
    column.items
      .filter(item => !item.external)
      .map(item => ({
        label: item.label,
        href: item.href,
        source: `footerColumns.${column.title}`,
      }))
  );
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
    expectRouteBackedLinks([...layoutLinks(), ...footerInternalLinks()]);
  });

  it("keeps footer columns focused and external links explicit", () => {
    const columnTitles = footerColumns.map(column => column.title);
    const footerLabels = footerColumns.flatMap(column =>
      column.items.map(item => `${column.title}:${item.label}`)
    );

    expect(new Set(columnTitles).size).toBe(columnTitles.length);
    expect(new Set(footerLabels).size).toBe(footerLabels.length);
    for (const column of footerColumns) {
      for (const item of column.items) {
        expect(item.label.trim(), `${column.title}:${item.href}`).toBe(item.label);
        expect(item.label.length, `${column.title}:${item.href}`).toBeGreaterThan(1);
        if (item.external) {
          const parsed = new URL(item.href);
          expect(parsed.protocol, item.label).toBe("https:");
          continue;
        }
        expect(item.href.startsWith("/"), item.label).toBe(true);
        expect(item.href.endsWith(".md"), item.label).toBe(false);
        expect(item.href.endsWith(".mdx"), item.label).toBe(false);
      }
    }
  });

  it("uses one canonical GitHub URL across layout and footer resources", () => {
    const externalFooterLinks = footerColumns.flatMap(column =>
      column.items.filter(item => item.external)
    );

    expect(baseOptions.githubUrl).toBe(siteConfig.githubUrl);
    expect(externalFooterLinks).toEqual([
      { label: "GitHub", href: siteConfig.githubUrl, external: true },
    ]);
  });
});
