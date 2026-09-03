import path from "node:path";
import { promises as fs } from "node:fs";
import { fileURLToPath } from "node:url";
import { generateFiles } from "fumadocs-openapi";
import { createOpenAPI } from "fumadocs-openapi/server";
import { API_TAG_ICONS } from "../lib/docs-icons";
import { COMPOZY_OPENAPI_ID, COMPOZY_OPENAPI_PATH } from "../lib/openapi";
import { API_SECTIONS } from "../lib/docs-navigation";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT_DIR = path.resolve(HERE, "../content/docs/api");
const REPO_ROOT = path.resolve(HERE, "../../..");
const PRESERVE = new Set(["index.mdx"]);
const OPENAPI_METHODS = new Set(["get", "post", "patch", "put", "delete"]);

type OpenAPIDocument = {
  paths?: Record<string, Record<string, unknown>>;
};

type OpenAPIOperation = {
  tags?: string[];
  [key: string]: unknown;
};

type APIRoute = {
  method: string;
  path: string;
};

type OpenAPIOptions = NonNullable<Parameters<typeof createOpenAPI>[0]>;
type OpenAPIInput = NonNullable<OpenAPIOptions["input"]>;
type OpenAPISchemaRecord = Exclude<OpenAPIInput, string[]>;
type OpenAPISchemaValue = OpenAPISchemaRecord[string];
type ResolveSchemaValue<T> = T extends () => infer Output ? Awaited<Output> : T;
type OpenAPISchemaOutput = ResolveSchemaValue<OpenAPISchemaValue>;
type FumadocsDocument = Exclude<OpenAPISchemaOutput, string>;

let referenceDocument: OpenAPIDocument | null = null;

async function cleanGenerated(): Promise<void> {
  const entries = await fs.readdir(OUT_DIR);
  const removals: Array<Promise<void>> = [];
  for (const entry of entries) {
    if (entry.endsWith(".mdx") && !PRESERVE.has(entry)) {
      removals.push(fs.rm(path.join(OUT_DIR, entry), { force: true }));
    }
  }
  await Promise.all(removals);
}

async function readRepoFile(...parts: string[]): Promise<string> {
  return fs.readFile(path.resolve(REPO_ROOT, ...parts), "utf8");
}

async function listRouteSourcePaths(dir: string): Promise<string[]> {
  const entries = await fs.readdir(path.resolve(REPO_ROOT, dir));
  return entries
    .filter(entry => entry === "routes.go" || entry.endsWith("_routes.go"))
    .sort()
    .map(entry => path.join(dir, entry));
}

function joinRoute(left: string, right: string): string {
  if (!right) {
    return left || "/";
  }
  return `${left.replace(/\/$/, "")}/${right.replace(/^\//, "")}`;
}

async function extractRegisteredRoutes(sourcePath: string): Promise<APIRoute[]> {
  const routes: APIRoute[] = [];
  const source = await readRepoFile(sourcePath);
  const groups = new Map<string, string>([["api", "/api"]]);
  const assignmentMatcher = /^\s*(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"/;
  const methodMatcher = /^\s*(\w+)\.(GET|POST|PATCH|PUT|DELETE)\("([^"]*)"/;

  for (const line of source.split("\n")) {
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
        });
      }
    }
  }

  return routes;
}

async function implementedRoutes(): Promise<APIRoute[]> {
  const [httpSources, udsSources] = await Promise.all([
    listRouteSourcePaths("internal/api/httpapi"),
    listRouteSourcePaths("internal/api/udsapi"),
  ]);
  const routeGroups = await Promise.all(
    [...httpSources, ...udsSources].map(sourcePath => extractRegisteredRoutes(sourcePath))
  );
  return routeGroups.flat();
}

function routePattern(route: string): RegExp {
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

function isCoveredByRegisteredRoute(openapiPath: string, method: string, routes: APIRoute[]) {
  const upperMethod = method.toUpperCase();
  return routes.some(
    route => route.method === upperMethod && routePattern(route.path).test(openapiPath)
  );
}

function isOpenAPIOperation(method: string, value: unknown): value is OpenAPIOperation {
  return OPENAPI_METHODS.has(method) && typeof value === "object" && value !== null;
}

function filterUnimplementedRoutes(doc: OpenAPIDocument, routes: APIRoute[]): OpenAPIDocument {
  for (const [openapiPath, pathItem] of Object.entries(doc.paths ?? {})) {
    for (const [method, operation] of Object.entries(pathItem)) {
      if (!isOpenAPIOperation(method, operation)) {
        continue;
      }
      if (!isCoveredByRegisteredRoute(openapiPath, method, routes)) {
        delete pathItem[method];
      }
    }

    const hasOperation = Object.entries(pathItem).some(([method, operation]) =>
      isOpenAPIOperation(method, operation)
    );
    if (!hasOperation) {
      delete doc.paths?.[openapiPath];
    }
  }
  return doc;
}

async function loadReferenceDocument(): Promise<OpenAPIDocument> {
  if (referenceDocument) {
    return referenceDocument;
  }
  const [raw, routes] = await Promise.all([
    fs.readFile(COMPOZY_OPENAPI_PATH, "utf8"),
    implementedRoutes(),
  ]);
  referenceDocument = filterUnimplementedRoutes(JSON.parse(raw) as OpenAPIDocument, routes);
  return referenceDocument;
}

const referenceOpenAPI = createOpenAPI({
  input: { [COMPOZY_OPENAPI_ID]: async () => (await loadReferenceDocument()) as FumadocsDocument },
});

async function readUsedTags(): Promise<string[]> {
  const doc = await loadReferenceDocument();
  const tags = new Set<string>();
  for (const ops of Object.values(doc.paths ?? {})) {
    for (const op of Object.values(ops)) {
      if (!isOpenAPIOperation("get", op)) continue;
      for (const tag of op.tags ?? []) tags.add(tag);
    }
  }
  return [...tags];
}

function tagSlug(tag: string): string {
  return tag.toLowerCase().replace(/\s+/g, "-");
}

function buildMetaPages(usedTagSlugs: Set<string>): string[] {
  const placed = new Set<string>();
  const pages: string[] = ["index"];
  for (const section of API_SECTIONS) {
    const present = section.ids.filter(id => usedTagSlugs.has(id));
    if (present.length === 0) continue;
    pages.push(`---${section.label}---`);
    for (const id of present) {
      pages.push(id);
      placed.add(id);
    }
  }
  const trailing = [...usedTagSlugs].filter(slug => !placed.has(slug)).sort();
  if (trailing.length > 0) {
    pages.push("---More---", ...trailing);
  }
  return pages;
}

async function writeMeta(usedTagSlugs: Set<string>): Promise<void> {
  // No `"root": true`: the reference lives as a collapsed folder inside the single /docs tree (D3).
  const meta = {
    title: "API Reference",
    icon: "FileCode",
    pages: buildMetaPages(usedTagSlugs),
  };
  await fs.writeFile(path.join(OUT_DIR, "meta.json"), `${JSON.stringify(meta, null, 4)}\n`, "utf8");
}

function iconForTitle(title: string): string | undefined {
  return API_TAG_ICONS[tagSlug(title)];
}

async function main(): Promise<void> {
  await cleanGenerated();
  await generateFiles({
    input: referenceOpenAPI,
    output: OUT_DIR,
    per: "tag",
    includeDescription: true,
    addGeneratedComment: true,
    frontmatter: (title, description) => {
      const frontmatter: Record<string, unknown> = {
        title,
        description: description ?? `CompozyOS ${title} HTTP endpoints.`,
        full: true,
        _generated: "fumadocs-openapi",
      };
      const icon = iconForTitle(title);
      if (icon) frontmatter.icon = icon;
      return frontmatter;
    },
  });
  const usedTags = (await readUsedTags()).map(tagSlug);
  await writeMeta(new Set(usedTags));
}

main().catch(err => {
  console.error("[generate-openapi] failed:", err);
  process.exit(1);
});
