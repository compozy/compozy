import { readdirSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

export type APIRoute = {
  method: string;
  path: string;
  source: string;
};

type RouteHelper = { router: string; body: string };

function listRouteSourcePaths(repoRoot: string, dir: string): string[] {
  return readdirSync(resolve(repoRoot, dir))
    .filter(entry => entry === "routes.go" || entry.endsWith("_routes.go"))
    .sort()
    .map(entry => `${dir}/${entry}`);
}

function joinRoute(left: string, right: string): string {
  if (!right) {
    return left || "/";
  }
  return `${left.replace(/\/$/, "")}/${right.replace(/^\//, "")}`;
}

export function readRouteHelpers(source: string, prefix = ""): Map<string, RouteHelper> {
  const helpers = new Map<string, RouteHelper>();
  for (const match of source.matchAll(
    /^func (\w+)\((\w+) gin\.IRouter[^)]*\) \{\n([\s\S]*?)^\}/gm
  )) {
    const [, name, router, body] = match;
    if (name && router && body !== undefined) helpers.set(`${prefix}${name}`, { router, body });
  }
  return helpers;
}

export function extractRegisteredRoutes(
  sourcePath: string,
  source: string,
  sharedHelpers: ReadonlyMap<string, RouteHelper> = new Map()
): APIRoute[] {
  const routes: APIRoute[] = [];
  const helpers = readRouteHelpers(source);
  const assignmentMatcher = /^\s*(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"/;
  const methodMatcher = /^\s*(\w+)\.(GET|POST|PATCH|PUT|DELETE)\("([^"]*)"/;
  const helperMatcher = /^\s*((?:\w+\.)?\w+)\((\w+),/;

  function visit(body: string, groups: Map<string, string>, packagePrefix = "") {
    for (const line of body.split("\n")) {
      const assignment = line.match(assignmentMatcher);
      if (assignment) {
        const [, target, parent, suffix] = assignment;
        const parentPath = groups.get(parent ?? "");
        if (target && parentPath !== undefined) {
          groups.set(target, joinRoute(parentPath, suffix ?? ""));
        }
        continue;
      }

      const method = line.match(methodMatcher);
      if (method) {
        const [, group, verb, suffix] = method;
        const prefix = groups.get(group ?? "");
        if (prefix !== undefined && verb) {
          routes.push({
            method: verb,
            path: joinRoute(prefix, suffix ?? ""),
            source: sourcePath,
          });
        }
        continue;
      }

      const call = line.match(helperMatcher);
      if (call) {
        const name = call[1] ?? "";
        const helperName = name.includes(".") ? name : `${packagePrefix}${name}`;
        const helper = helpers.get(helperName) ?? sharedHelpers.get(helperName);
        const prefix = groups.get(call[2] ?? "");
        if (helper && prefix !== undefined) {
          const helperPackagePrefix = helperName.slice(0, helperName.lastIndexOf(".") + 1);
          visit(helper.body, new Map([[helper.router, prefix]]), helperPackagePrefix);
        }
      }
    }
  }

  visit(source, new Map([["api", "/api"]]));
  return routes;
}

export function readRegisteredAPIRoutes(repoRoot: string): APIRoute[] {
  const sharedHelpers = new Map(
    listRouteSourcePaths(repoRoot, "internal/api/core").flatMap(path => [
      ...readRouteHelpers(readFileSync(resolve(repoRoot, path), "utf8"), "core."),
    ])
  );
  return [
    ...listRouteSourcePaths(repoRoot, "internal/api/httpapi"),
    ...listRouteSourcePaths(repoRoot, "internal/api/udsapi"),
  ].flatMap(sourcePath =>
    extractRegisteredRoutes(
      sourcePath,
      readFileSync(resolve(repoRoot, sourcePath), "utf8"),
      sharedHelpers
    )
  );
}

// The full docs scan compares each registered route many times; compile each pattern once.
const routePatterns = new Map<string, RegExp>();

export function routePattern(route: string): RegExp {
  const cached = routePatterns.get(route);
  if (cached) return cached;

  const escaped = route
    .split("/")
    .map(part => {
      if (part.startsWith(":")) {
        return "[^/]+";
      }
      if (part.startsWith("*")) {
        return ".*";
      }
      return part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    })
    .join("/");
  const pattern = new RegExp(`^${escaped}$`);
  routePatterns.set(route, pattern);
  return pattern;
}
