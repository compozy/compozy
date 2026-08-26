/**
 * Single source of truth for reading the daemon's registered Gin routes out of
 * `internal/api/{httpapi,udsapi}/*_routes.go`. Both the OpenAPI reference generator and the
 * documented-route test read it, so a route the daemon serves can never look unimplemented to one
 * of them and implemented to the other.
 */

export type APIRoute = { method: string; path: string };

const HTTP_METHODS = "GET|POST|PATCH|PUT|DELETE";
const ASSIGNMENT_MATCHER = /^\s*(\w+)\s*:=\s*(\w+)\.Group\(\s*"([^"]*)"/;
const METHOD_MATCHER = new RegExp(`^\\s*(\\w+)\\.(${HTTP_METHODS})\\(\\s*"([^"]*)"`);
/** `registerScopedCallRoutes(api.Group("/workspaces/:workspace_id"), handlers)` */
const HELPER_GROUP_CALL_MATCHER = /^\s*(\w+)\(\s*(\w+)\.Group\(\s*"([^"]*)"\s*\)/;
/** `registerCallRoutes(api, handlers)` */
const HELPER_PLAIN_CALL_MATCHER = /^\s*(\w+)\(\s*(\w+)\s*[,)]/;
/** `func registerScopedCallRoutes(scope gin.IRouter, handlers *Handlers) {` */
const HELPER_DEFINITION_MATCHER = /^func\s+(\w+)\(\s*(\w+)\s+(?:gin\.IRouter|\*gin\.RouterGroup)\b/;

const MAX_HELPER_DEPTH = 4;

type Helper = { param: string; body: string[] };

/** Collapse only expressions with open parentheses; braces remain line boundaries. */
function logicalLines(source: string): string[] {
  const logical: string[] = [];
  let current = "";
  let depth = 0;
  let quoted = false;
  let escaped = false;

  for (const rawLine of source.split("\n")) {
    const line = rawLine.trim();
    current = current === "" ? line : `${current} ${line}`;

    for (let index = 0; index < rawLine.length; index += 1) {
      const char = rawLine[index];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (char === "\\" && quoted) {
        escaped = true;
        continue;
      }
      if (char === '"') {
        quoted = !quoted;
        continue;
      }
      if (quoted) continue;
      if (char === "/" && rawLine[index + 1] === "/") break;
      if (char === "(") depth += 1;
      if (char === ")") depth -= 1;
    }

    if (depth <= 0) {
      if (current !== "") logical.push(current);
      current = "";
      depth = 0;
    }
  }

  if (current !== "") logical.push(current);
  return logical;
}

export function joinRoute(left: string, right: string): string {
  if (!right) {
    return left || "/";
  }
  return `${left.replace(/\/$/, "")}/${right.replace(/^\//, "")}`;
}

export function routePattern(route: string): RegExp {
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
  return new RegExp(`^${escaped}$`);
}

export function isRouteSourceFile(entry: string): boolean {
  return entry === "routes.go" || entry.endsWith("_routes.go");
}

/** Indexes every `func name(param gin.IRouter, …)` in the file so call sites can be expanded. */
function collectHelpers(lines: string[]): Map<string, Helper> {
  const helpers = new Map<string, Helper>();
  for (let index = 0; index < lines.length; index += 1) {
    const definition = (lines[index] ?? "").match(HELPER_DEFINITION_MATCHER);
    if (!definition) continue;
    const [, name, param] = definition;
    const body: string[] = [];
    for (let cursor = index + 1; cursor < lines.length; cursor += 1) {
      const line = lines[cursor] ?? "";
      if (line === "}") break;
      body.push(line);
    }
    if (name && param) helpers.set(name, { param, body });
  }
  return helpers;
}

function walk(
  lines: string[],
  groups: Map<string, string>,
  helpers: Map<string, Helper>,
  depth: number,
  routes: APIRoute[]
): void {
  for (const line of lines) {
    const assignment = line.match(ASSIGNMENT_MATCHER);
    if (assignment) {
      const [, target, parent, suffix] = assignment;
      const parentPath = groups.get(parent ?? "");
      if (target && parentPath !== undefined) {
        groups.set(target, joinRoute(parentPath, suffix ?? ""));
      }
      continue;
    }

    const method = line.match(METHOD_MATCHER);
    if (method) {
      const [, group, verb, suffix] = method;
      const prefix = groups.get(group ?? "");
      if (prefix !== undefined && verb) {
        routes.push({ method: verb, path: joinRoute(prefix, suffix ?? "") });
      }
      continue;
    }

    if (depth >= MAX_HELPER_DEPTH) continue;

    const groupCall = line.match(HELPER_GROUP_CALL_MATCHER);
    if (groupCall) {
      const [, name, receiver, suffix] = groupCall;
      const helper = helpers.get(name ?? "");
      const receiverPath = groups.get(receiver ?? "");
      if (helper && receiverPath !== undefined) {
        const scoped = new Map(groups);
        scoped.set(helper.param, joinRoute(receiverPath, suffix ?? ""));
        walk(helper.body, scoped, helpers, depth + 1, routes);
      }
      continue;
    }

    const plainCall = line.match(HELPER_PLAIN_CALL_MATCHER);
    if (plainCall) {
      const [, name, argument] = plainCall;
      const helper = helpers.get(name ?? "");
      const argumentPath = groups.get(argument ?? "");
      if (helper && argumentPath !== undefined) {
        const scoped = new Map(groups);
        scoped.set(helper.param, argumentPath);
        walk(helper.body, scoped, helpers, depth + 1, routes);
      }
    }
  }
}

/**
 * Reads every route a `*_routes.go` file registers. Route registration is often split into a helper
 * that takes the parent group as a parameter (one helper body, two mounted scopes); those call sites
 * are expanded with the parameter bound to the resolved prefix, so both scopes are reported.
 */
export function extractRegisteredRoutes(source: string): APIRoute[] {
  const lines = logicalLines(source);
  const routes: APIRoute[] = [];
  walk(lines, new Map([["api", "/api"]]), collectHelpers(lines), 0, routes);
  return routes;
}

export function isCoveredByRegisteredRoute(
  documentedRoute: string,
  routes: APIRoute[],
  method?: string
): boolean {
  const upperMethod = method?.toUpperCase();
  return routes.some(route => {
    if (upperMethod && route.method !== upperMethod) return false;
    return routePattern(route.path).test(documentedRoute);
  });
}
