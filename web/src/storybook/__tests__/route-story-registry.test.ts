import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";

import { describe, expect, it } from "vitest";

import {
  routeStoryExclusions,
  routeStoryRegistry,
  routeStoryRegistryByRoutePath,
  routeStorySystems,
  type GeneratedRoutePath,
} from "../route-story-registry";
import { storybookSystemHandlerGroups } from "../msw";

const WEB_SRC_ROOT = join(__dirname, "../..");
const ROUTE_TREE_PATH = join(WEB_SRC_ROOT, "routeTree.gen.ts");
const STORY_META_TITLE_PATTERN = /const\s+meta\b[\s\S]*?title:\s*"([^"]+)"/;
const ROUTE_TREE_FULL_PATHS_PATTERN = /fullPaths:\n(?<body>[\s\S]*?)\n\s+fileRoutesByTo:/;

function generatedRoutePaths(): GeneratedRoutePath[] {
  const source = readFileSync(ROUTE_TREE_PATH, "utf8");
  const body = ROUTE_TREE_FULL_PATHS_PATTERN.exec(source)?.groups?.body;
  if (!body) {
    throw new Error("Unable to locate FileRouteTypes.fullPaths in routeTree.gen.ts");
  }

  return Array.from(body.matchAll(/'([^']+)'/g), match => match[1] as GeneratedRoutePath);
}

function walkStories(dir: string, output: string[] = []) {
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry);
    const stats = statSync(fullPath);
    if (stats.isDirectory()) {
      walkStories(fullPath, output);
      continue;
    }
    if (entry.endsWith(".stories.tsx")) {
      output.push(fullPath);
    }
  }
  return output;
}

function storyTitles() {
  return walkStories(WEB_SRC_ROOT).flatMap(file => {
    const source = readFileSync(file, "utf8");
    const match = STORY_META_TITLE_PATTERN.exec(source);
    return match
      ? [
          {
            file: relative(WEB_SRC_ROOT, file),
            title: match[1],
          },
        ]
      : [];
  });
}

describe("route story registry", () => {
  it("covers every generated TanStack route or documents an explicit exclusion", () => {
    const exclusions = new Set(routeStoryExclusions.map(entry => entry.routePath));

    expect(routeStoryExclusions).toEqual([]);

    const uncovered = generatedRoutePaths().filter(
      routePath => !routeStoryRegistryByRoutePath.has(routePath) && !exclusions.has(routePath)
    );

    expect(uncovered).toEqual([]);
  });

  it("uses concrete app paths for every route story", () => {
    for (const entry of routeStoryRegistry) {
      expect(entry.storybookPath, entry.routePath).not.toContain("$");
      expect(entry.storybookPath, entry.routePath).toMatch(/^\//);
      expect(entry.title, entry.routePath).toMatch(/^systems\/[^/]+\/routes\/[^/]+$/);
      expect(entry.storyName, entry.routePath).toMatch(/^[A-Z]/);
    }
  });

  it("keeps all story titles under systems with component or route subtrees", () => {
    const invalidTitles = storyTitles().filter(({ title }) => {
      if (/^(components|routes|app)\//.test(title)) return true;
      if (!title.startsWith("systems/")) return true;
      if (title.includes("/routes/")) return false;
      return !title.includes("/components/");
    });

    expect(invalidTitles).toEqual([]);
  });

  it("keeps all web stories physically under systems", () => {
    const misplacedStories = walkStories(WEB_SRC_ROOT)
      .map(file => relative(WEB_SRC_ROOT, file))
      .filter(file => !file.startsWith("systems/"));

    expect(misplacedStories).toEqual([]);
  });

  it("registers an MSW group for every route-owning system", () => {
    const missingHandlerGroups = routeStorySystems.filter(
      system => !(system in storybookSystemHandlerGroups)
    );

    expect(missingHandlerGroups).toEqual([]);
  });
});
