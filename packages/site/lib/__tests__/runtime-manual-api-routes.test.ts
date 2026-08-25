import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  type APIRoute,
  extractRegisteredRoutes,
  isRouteSourceFile,
  routePattern,
} from "../api-routes";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const contentRoot = resolve(siteRoot, "content");

type ManualDoc = {
  path: string;
  content: string;
};

const ignoredExternalPrefixes = ["/api/v1"];

function readRepoFile(...parts: string[]): string {
  return readFileSync(resolve(repoRoot, ...parts), "utf8");
}

function listRouteSourcePaths(dir: string): string[] {
  return readdirSync(resolve(repoRoot, dir))
    .filter(isRouteSourceFile)
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

function implementedRoutes(): APIRoute[] {
  return [
    ...listRouteSourcePaths("internal/api/httpapi"),
    ...listRouteSourcePaths("internal/api/udsapi"),
  ].flatMap(sourcePath => extractRegisteredRoutes(readRepoFile(sourcePath)));
}

function normalizeDocumentedRoute(raw: string): string {
  const withoutHost = raw.replace(/^https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?/, "");
  const withoutQuery = withoutHost.split(/[?#]/, 1)[0] ?? withoutHost;
  return withoutQuery.replace(/[)"'`,.;]+$/g, "").replace(/\/$/, "") || "/";
}

function extractDocumentedAPIRoutes(content: string): string[] {
  const routes = new Set<string>();
  // `(?<!\/docs)` keeps the generated reference URLs (`/docs/api/...`) out of the daemon-route scan.
  for (const match of content.matchAll(
    /(?:https?:\/\/(?:localhost|127\.0\.0\.1)(?::\d+)?)?(?<!\/docs)(\/api\/[A-Za-z0-9_:$<>{}./?-]+)/g
  )) {
    const normalized = normalizeDocumentedRoute(match[1] ?? "");
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
  it("expands helper-mounted routes for global and workspace scopes", () => {
    const source = `
func registerRoutes(api *gin.RouterGroup, handlers *Handlers) {
  registerCallRoutes(api, handlers)
  registerCallRoutes(api.Group("/workspaces/:workspace_id"), handlers)
}

func registerCallRoutes(scope gin.IRouter, handlers *Handlers) {
  scope.GET("/calls", handlers.CallsList)
}
`;

    expect(extractRegisteredRoutes(source)).toEqual([
      { method: "GET", path: "/api/calls" },
      { method: "GET", path: "/api/workspaces/:workspace_id/calls" },
    ]);
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
