import { readdirSync, readFileSync, statSync } from "node:fs";
import { dirname, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  extractRegisteredRoutes,
  readRegisteredAPIRoutes,
  readRouteHelpers,
  routePattern,
  type APIRoute,
} from "../registered-api-routes";

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const repoRoot = resolve(siteRoot, "../..");
const contentRoot = resolve(siteRoot, "content");

type ManualDoc = {
  path: string;
  content: string;
};

const ignoredExternalPrefixes = ["/api/v1"];

// Retired endpoints appear only as migration references.
const retiredMemoryRoutes =
  /^\/api\/(?:memory|settings\/memory|workspaces\/\{[^}]+\}\/memory)(?:\/|$)/;
const retiredGatewayBindingRoute = "/api/gateway/ingress-bindings";

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
    const sharedHelpers = readRouteHelpers(
      `
func RegisterSessionReadRoutes(reads gin.IRouter, handlers *BaseHandlers, mw ...gin.HandlerFunc) {
  reads.GET("/:session_id/status", handlers.GetSessionStatus)
}
`,
      "core."
    );
    const routes = extractRegisteredRoutes(
      "session_routes.go",
      `
func registerSessionRoutes(api gin.IRouter, handlers *Handlers) {
  sessions := api.Group("/sessions")
  core.RegisterSessionReadRoutes(sessions, handlers.BaseHandlers, handlers.guard())
  workspaceSessions := api.Group("/workspaces/:workspace_id/sessions")
  registerSessionReadRoutes(workspaceSessions, handlers)
}
func registerSessionReadRoutes(reads gin.IRouter, handlers *Handlers) {
  reads.GET("/:session_id/status", handlers.GetSessionStatus)
}
`,
      sharedHelpers
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
    expect(
      isCoveredByRegisteredRoute("/api/missing-route", readRegisteredAPIRoutes(repoRoot))
    ).toBe(false);
  });

  it("points documented CompozyOS /api routes at implemented HTTP or UDS handlers", () => {
    const registeredRoutes = readRegisteredAPIRoutes(repoRoot);
    const violations = listManualDocs(contentRoot).flatMap(doc =>
      extractDocumentedAPIRoutes(doc.content)
        .filter(route => !isCoveredByRegisteredRoute(route, registeredRoutes))
        .filter(
          route =>
            !(
              doc.path.startsWith("docs/migration/") &&
              (retiredMemoryRoutes.test(route) || route === retiredGatewayBindingRoute)
            )
        )
        .map(route => `${doc.path} -> ${route}`)
    );

    expect(violations).toEqual([]);
  });
});
