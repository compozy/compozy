import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const contentRoot = resolve(siteRoot, "content");

type ManualDoc = {
  path: string;
  content: string;
};

type APIRoute = {
  method: string;
  path: string;
  source: string;
};

const ignoredExternalPrefixes = ["/api/v1"];

function readRepoFile(...parts: string[]): string {
  return readFileSync(resolve(repoRoot, ...parts), "utf8");
}

function listRouteSourcePaths(dir: string): string[] {
  return readdirSync(resolve(repoRoot, dir))
    .filter(entry => entry === "routes.go" || entry.endsWith("_routes.go"))
    .sort()
    .map(entry => `${dir}/${entry}`);
}

function listManualDocs(dir: string): ManualDoc[] {
  const docs: ManualDoc[] = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = resolve(dir, entry);
    const stat = statSync(fullPath);
    if (stat.isDirectory()) {
      docs.push(...listManualDocs(fullPath));
      continue;
    }
    if (stat.isFile() && fullPath.endsWith(".mdx")) {
      docs.push({
        path: relative(contentRoot, fullPath),
        content: readFileSync(fullPath, "utf8"),
      });
    }
  }
  return docs.sort((left, right) => left.path.localeCompare(right.path));
}

function joinRoute(left: string, right: string): string {
  if (!right) {
    return left || "/";
  }
  return `${left.replace(/\/$/, "")}/${right.replace(/^\//, "")}`;
}

function extractRegisteredRoutes(
  sourcePath: string,
  source = readRepoFile(sourcePath)
): APIRoute[] {
  const routes: APIRoute[] = [];
  const helpers = new Map<string, { router: string; body: string }>();
  for (const match of source.matchAll(
    /^func (\w+)\((\w+) gin\.IRouter[^)]*\) \{\n([\s\S]*?)^\}/gm
  )) {
    const [, name, router, body] = match;
    if (name && router && body !== undefined) helpers.set(name, { router, body });
  }
  const assignmentMatcher = /^\s*(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"/;
  const methodMatcher = /^\s*(\w+)\.(GET|POST|PATCH|PUT|DELETE)\("([^"]*)"/;
  const helperMatcher = /^\s*(\w+)\((\w+),/;

  function visit(body: string, groups: Map<string, string>) {
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
        const helper = helpers.get(call[1] ?? "");
        const prefix = groups.get(call[2] ?? "");
        if (helper && prefix !== undefined) {
          visit(helper.body, new Map([[helper.router, prefix]]));
        }
      }
    }
  }

  visit(source, new Map([["api", "/api"]]));
  return routes;
}

function implementedRoutes(): APIRoute[] {
  return [
    ...listRouteSourcePaths("internal/api/httpapi"),
    ...listRouteSourcePaths("internal/api/udsapi"),
  ].flatMap(sourcePath => extractRegisteredRoutes(sourcePath));
}

// The full docs scan compares each registered route many times; compile each pattern once.
const routePatterns = new Map<string, RegExp>();

function routePattern(route: string): RegExp {
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

function normalizeDocumentedRoute(raw: string): string {
  const withoutHost = raw.replace(/^https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?/, "");
  const withoutQuery = withoutHost.split(/[?#]/, 1)[0] ?? withoutHost;
  return withoutQuery.replace(/[)"'`,.;]+$/g, "").replace(/\/$/, "") || "/";
}

function extractDocumentedAPIRoutes(content: string): string[] {
  const routes = new Set<string>();
  // Consume absolute URLs whole so another site's /api path cannot be mistaken for ours.
  // `(?<!\/docs)` keeps generated reference links out of the relative daemon-route scan.
  for (const match of content.matchAll(
    /https?:\/\/[^\s"'`<>()]+|(?<!\/docs)\/api\/[A-Za-z0-9_:$<>{}./?-]+/g
  )) {
    let raw = match[0];
    if (/^https?:\/\//.test(raw)) {
      const url = new URL(raw);
      if (!["localhost", "127.0.0.1", "0.0.0.0", "[::1]", "compozy.com"].includes(url.hostname)) {
        continue;
      }
      raw = url.pathname;
    }
    const normalized = normalizeDocumentedRoute(raw);
    if (
      normalized.startsWith("/api/") &&
      !ignoredExternalPrefixes.some(prefix => normalized.startsWith(prefix))
    ) {
      routes.add(normalized);
    }
  }
  return [...routes].sort();
}

function isCoveredByRegisteredRoute(
  documentedRoute: string,
  registeredRoutes: APIRoute[]
): boolean {
  return registeredRoutes.some(
    route =>
      routePattern(route.path).test(documentedRoute) || route.path.startsWith(`${documentedRoute}/`)
  );
}

describe("manual API route references", () => {
  // Invariant: route registration delegates preserve each caller's Gin prefix.
  // Owner: this manual-doc contract scanner; live HTTP/UDS tests own runtime routing.
  it("resolves shared route helpers at both Global and workspace prefixes", () => {
    const routes = extractRegisteredRoutes(
      "session_routes.go",
      `
func registerSessionRoutes(api gin.IRouter, handlers *Handlers) {
  sessions := api.Group("/sessions")
  registerSessionReadRoutes(sessions, handlers)
  workspaceSessions := api.Group("/workspaces/:workspace_id/sessions")
  registerSessionReadRoutes(workspaceSessions, handlers)
}
func registerSessionReadRoutes(reads gin.IRouter, handlers *Handlers) {
  reads.GET("/:session_id/status", handlers.GetSessionStatus)
}
`
    );
    expect(routes.map(route => route.path)).toEqual([
      "/api/sessions/:session_id/status",
      "/api/workspaces/:workspace_id/sessions/:session_id/status",
    ]);
    expect(isCoveredByRegisteredRoute("/api/sessions/sess_1/missing", routes)).toBe(false);
  });

  it("distinguishes external API citations from local daemon routes", () => {
    const routes = extractDocumentedAPIRoutes(`
      [Stripe](https://docs.stripe.com/api/idempotent_requests)
      [Nested external API](https://example.org/reference/api/unknown)
      [Generated reference](/docs/api/sessions)
      GET /api/sessions
      GET http://localhost:4318/api/sessions/sess_1
      GET https://compozy.com/api/missing-route
    `);

    expect(routes).toEqual(["/api/missing-route", "/api/sessions", "/api/sessions/sess_1"]);
    expect(isCoveredByRegisteredRoute("/api/missing-route", implementedRoutes())).toBe(false);
  });

  it("points documented CompozyOS /api routes at implemented HTTP or UDS handlers", () => {
    const registeredRoutes = implementedRoutes();
    const violations = listManualDocs(contentRoot).flatMap(doc =>
      extractDocumentedAPIRoutes(doc.content)
        .filter(route => !isCoveredByRegisteredRoute(route, registeredRoutes))
        .map(route => `${doc.path} -> ${route}`)
    );

    expect(violations).toEqual([]);
  });
});
