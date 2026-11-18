/**
 * Builds the SDK reference from the OpenAPI document, the SDK samples, and the two clients' sources.
 * Pure: `scripts/generate-sdk-reference.ts` does the file IO, tests feed the same inputs.
 */
import type { SDKSampleManifest } from "@/lib/sdk-samples";
import {
  countOf,
  kebabCase,
  type SdkField,
  type SdkGoCall,
  type SdkKind,
  type SdkMethod,
  type SdkReference,
  type SdkResource,
  type SdkTsCall,
} from "@/lib/sdk-reference-model";

type Schema = {
  $ref?: string;
  type?: string;
  enum?: unknown[];
  items?: Schema;
  oneOf?: Schema[];
  anyOf?: Schema[];
};

type Parameter = {
  name: string;
  in: string;
  required?: boolean;
  description?: string;
  schema?: Schema;
};

type Operation = {
  operationId?: string;
  summary?: string;
  parameters?: Parameter[];
};

export type OpenAPIDocument = {
  paths?: Record<string, Record<string, unknown>>;
};

export interface SdkReferenceInputs {
  openapi: OpenAPIDocument;
  samples: SDKSampleManifest;
  /** Every non-test `.go` file of `sdk/go/client`, concatenated. */
  goSource: string;
  /** `sdk/client/src/generated/sdk/types.gen.ts`: the resource methods. */
  tsTypes: string;
  /** `sdk/client/src/generated/sse/types.gen.ts`: the operations `compozy.events()` reads. */
  tsSseTypes: string;
  /** `sdk/client/src/prompt.ts`: the `PromptInput` that `compozy.prompt()` takes. */
  tsPrompt: string;
}

const HTTP_METHODS = new Set(["get", "post", "put", "patch", "delete"]);
const PARAM_PLACES = new Set(["path", "query", "header"]);

/** A schema as a short type: `$ref` names, `string[]`, enums as their literal union. */
export function schemaType(schema: Schema | undefined): string {
  if (!schema) return "any";
  if (schema.$ref) return schema.$ref.split("/").at(-1) ?? "object";
  const union = schema.oneOf ?? schema.anyOf;
  if (union) return union.map(schemaType).join(" | ");
  if (schema.type === "array") return `${schemaType(schema.items)}[]`;
  if (schema.enum) return schema.enum.map(value => JSON.stringify(value)).join(" | ");
  return schema.type ?? "object";
}

function pascalCase(id: string): string {
  return id.charAt(0).toUpperCase() + id.slice(1);
}

/** `func (c *Client) Name(ctx context.Context, params P, body B) (*R, error)` → name, args, returns. */
export function parseGoMethods(source: string): Map<string, SdkGoCall> {
  const methods = new Map<string, SdkGoCall>();
  const matcher = /^func \(c \*Client\) (\w+)\(ctx context\.Context(?:, ([^)]*))?\) (.+?) \{$/gm;
  for (const match of source.matchAll(matcher)) {
    const [, name = "", rawArgs = "", rawResults = ""] = match;
    const args = rawArgs
      .split(",")
      .map(arg => arg.trim().split(/\s+/).at(-1) ?? "")
      .filter(Boolean);
    const results = rawResults
      .replace(/^\(|\)$/g, "")
      .split(",")
      .map(result => result.trim().split(/\s+/).at(-1) ?? "");
    const returns = results.length > 1 ? (results[0] ?? "error") : "error";
    methods.set(name, { name, args, returns });
  }
  return methods;
}

/** The body of `export type <name> = { … };`, top-level keys only. */
function typeBlock(source: string, name: string): Map<string, string> | null {
  const start = source.indexOf(`export type ${name} = {\n`);
  if (start === -1) return null;
  const end = source.indexOf("\n};", start);
  const keys = new Map<string, string>();
  const lines = source.slice(start, end).split("\n").slice(1);
  for (let index = 0; index < lines.length; index += 1) {
    const match = /^ {2}(\w+)\??: (.*)$/.exec(lines[index] ?? "");
    if (!match) continue;
    const [, key = "", rest = ""] = match;
    // A multi-line object type keeps its first line; callers fall back to the exported alias.
    keys.set(key, rest.endsWith(";") ? rest.slice(0, -1) : "{");
  }
  return keys;
}

/** Options, body and response type the TypeScript resource method takes and resolves to. */
export function tsCallOf(tsTypes: string, operationId: string): SdkTsCall | null {
  const name = pascalCase(operationId);
  const data = typeBlock(tsTypes, `${name}Data`);
  if (!data) return null;
  const options = ["path", "query", "headers", "body"].filter(key => {
    const type = data.get(key);
    return type !== undefined && type !== "never";
  });
  const bodyType = data.get("body");
  const body =
    bodyType === undefined || bodyType === "never"
      ? ""
      : bodyType === "{"
        ? `${name}Data["body"]`
        : bodyType;
  const responses = typeBlock(tsTypes, `${name}Responses`);
  const codes = [...(responses?.entries() ?? [])].filter(([code]) => code.startsWith("2"));
  let returns = "void";
  if (codes.length === 1 && codes[0]?.[1] !== "{") returns = codes[0]?.[1] ?? "void";
  else if (codes.length > 0) returns = `${name}Response`;
  return { options, body, returns };
}

/**
 * The option keys of `export type <name> = { … };`, as `<group>:<key>` for its nested `path`,
 * `query` and `headers` object types.
 */
function nestedKeys(source: string, name: string): Set<string> {
  const keys = new Set<string>();
  const start = source.indexOf(`export type ${name} = {\n`);
  if (start === -1) return keys;
  let group: string | null = null;
  for (const line of source.slice(start, source.indexOf("\n};", start)).split("\n")) {
    const opener = /^ {2}(\w+)\??: \{$/.exec(line);
    if (opener) group = opener[1] ?? null;
    else if (/^ {2}\S/.test(line)) group = null;
    // A union type may continue on the next lines (`status?:` then `| "draft"`).
    const match = group ? /^ {4}"?([\w-]+)"?\??:(?: |$)/.exec(line) : null;
    if (match?.[1]) keys.add(`${group}:${match[1]}`);
  }
  return keys;
}

/** Field names of `<keyword> <name> {` … `}` (a TS interface or a Go struct), `,`-lists split. */
function declaredFields(source: string, opener: string): string[] {
  const start = source.indexOf(`${opener} {\n`);
  if (start === -1) return [];
  const body = source.slice(start, source.indexOf("\n}", start)).split("\n").slice(1);
  return body.flatMap(line => {
    const ts = /^ {2}(\w+)\??:/.exec(line);
    if (ts?.[1]) return [ts[1]];
    const go = /^\t([\w, ]+?)\s+[\w.*[\]]+$/.exec(line);
    return go?.[1] ? go[1].split(",").map(name => name.trim()) : [];
  });
}

/** `url:"<http name>"` → Go field, for each field of `type <name> struct { … }`. */
function goParamFields(goSource: string, name: string): Map<string, string> {
  const fields = new Map<string, string>();
  const start = goSource.indexOf(`type ${name} struct {\n`);
  if (start === -1) return fields;
  const body = goSource.slice(start, goSource.indexOf("\n}", start));
  for (const match of body.matchAll(/^\t(\w+)\s+\S+\s+`url:"([^"]+)"`/gm)) {
    if (match[1] && match[2]) fields.set(match[2], match[1]);
  }
  return fields;
}

/** `workspace_id`, `workspaceId` and `WorkspaceID` name the same option. */
function sameOption(left: string, right: string): boolean {
  const flat = (name: string) => name.toLowerCase().replace(/[^a-z0-9]/g, "");
  return flat(left) === flat(right);
}

const TS_GROUP: Record<NonNullable<SdkField["in"]>, string> = {
  path: "path",
  query: "query",
  header: "headers",
};

/** Sets each option's name in the TypeScript and Go clients, `null` where a client lacks it. */
function nameOptions(
  fields: SdkField[],
  method: Pick<SdkMethod, "id" | "kind" | "go">,
  inputs: SdkReferenceInputs
): SdkField[] {
  if (method.kind === "websocket" || fields.length === 0) return fields;
  if (method.kind === "prompt") {
    // `compozy.prompt()` and `c.Prompt()` take a `PromptInput`, not the route's options.
    const ts = declaredFields(inputs.tsPrompt, "export interface PromptInput");
    const go = declaredFields(inputs.goSource, "type PromptInput struct");
    return fields.map(field => ({
      ...field,
      client: {
        ts: ts.find(name => sameOption(name, field.name)) ?? null,
        go: go.find(name => sameOption(name, field.name)) ?? null,
      },
    }));
  }
  const sse = method.kind === "sse";
  const tsKeys = nestedKeys(
    sse ? inputs.tsSseTypes : inputs.tsTypes,
    `${pascalCase(method.id)}Data`
  );
  const goArg = method.go?.args.find(arg => arg.endsWith("Params"));
  const goFields = goArg ? goParamFields(inputs.goSource, goArg) : new Map<string, string>();
  return fields.map(field => {
    const place = field.in ?? "query";
    // `compozy.events()` forwards no headers (`SseOptions` keeps `path`, `query` and `body`).
    const tsTakes = !(sse && place === "header") && tsKeys.has(`${TS_GROUP[place]}:${field.name}`);
    return {
      ...field,
      client: { ts: tsTakes ? field.name : null, go: goFields.get(field.name) ?? null },
    };
  });
}

/** `events()` forwards only `path`, `query` and `body` of the generated SSE options (`SseOptions`). */
function sseCallOf(tsSseTypes: string, operationId: string): SdkTsCall | null {
  const call = tsCallOf(tsSseTypes, operationId);
  if (!call) return null;
  return { ...call, options: call.options.filter(option => option !== "headers") };
}

/**
 * The Go method for an operation. When the client also generates `<Name>WithParams` (the plain
 * `<Name>` then calls it with empty params), the full form is the one that takes every option.
 */
function goMethodOf(methods: Map<string, SdkGoCall>, operationId: string): SdkGoCall | null {
  const name = pascalCase(operationId);
  return methods.get(`${name}WithParams`) ?? methods.get(name) ?? null;
}

function operationsById(document: OpenAPIDocument) {
  const operations = new Map<string, { method: string; path: string; operation: Operation }>();
  for (const [path, item] of Object.entries(document.paths ?? {})) {
    for (const [method, value] of Object.entries(item)) {
      if (!HTTP_METHODS.has(method) || !value || typeof value !== "object") continue;
      const operation = value as Operation;
      if (operation.operationId) {
        operations.set(operation.operationId, { method: method.toUpperCase(), path, operation });
      }
    }
  }
  return operations;
}

function paramsOf(operation: Operation): SdkField[] {
  const fields: SdkField[] = [];
  for (const param of operation.parameters ?? []) {
    if (!PARAM_PLACES.has(param.in)) continue;
    fields.push({
      name: param.name,
      in: param.in as SdkField["in"],
      type: schemaType(param.schema),
      required: Boolean(param.required),
      description: param.description ?? "",
      client: { ts: null, go: null },
    });
  }
  return fields;
}

function resourceTitle(id: string): string {
  const words = kebabCase(id).replaceAll("-", " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

function byId(left: { id: string }, right: { id: string }): number {
  return left.id < right.id ? -1 : left.id > right.id ? 1 : 0;
}

/**
 * One resource per `resource` of the SDK samples, ordered by method count (then id); methods ordered
 * by operation id. Every sampled operation must exist in the OpenAPI document.
 */
export function buildSdkReference(inputs: SdkReferenceInputs): SdkReference {
  const operations = operationsById(inputs.openapi);
  const goMethods = parseGoMethods(inputs.goSource);
  const grouped = new Map<string, SdkMethod[]>();
  for (const [id, sample] of Object.entries(inputs.samples.operations)) {
    const entry = operations.get(id);
    if (!entry) throw new Error(`sdk-samples.json names ${id}, which openapi/compozy.json lacks.`);
    const kind = sample.kind as SdkKind;
    const realtime = kind === "websocket" || kind === "prompt";
    const ts = realtime
      ? null
      : kind === "sse"
        ? sseCallOf(inputs.tsSseTypes, id)
        : tsCallOf(inputs.tsTypes, id);
    const go = kind === "prompt" ? (goMethods.get("Prompt") ?? null) : goMethodOf(goMethods, id);
    const method: SdkMethod = {
      id,
      summary: entry.operation.summary ?? "",
      method: entry.method,
      path: entry.path,
      kind,
      app: sample.app_access === "allowed",
      params: nameOptions(paramsOf(entry.operation), { id, kind, go }, inputs),
      ts,
      go,
      samples: { ts: sample.typescript ?? "", go: sample.go ?? "" },
    };
    const list = grouped.get(sample.resource) ?? [];
    list.push(method);
    grouped.set(sample.resource, list);
  }
  const resources: SdkResource[] = [...grouped].map(([id, ops]) => ({
    id,
    slug: kebabCase(id),
    title: resourceTitle(id),
    ops: ops.toSorted(byId),
  }));
  resources.sort((left, right) => right.ops.length - left.ops.length || byId(left, right));
  const all = resources.flatMap(resource => resource.ops);
  return {
    stats: {
      operations: all.length,
      resources: resources.length,
      app: all.filter(method => method.app).length,
      streams: all.filter(method => method.kind === "sse").length,
    },
    resources,
  };
}

export const SDK_GENERATED_MARKER = "sdk-reference";
/** Hand-written pages in `content/docs/sdk/`, in sidebar order after the overview. */
export const SDK_GUIDE_PAGES = ["client", "prompt", "events", "errors"] as const;

function yamlString(value: string): string {
  return JSON.stringify(value);
}

/** The generated stub: frontmatter and one `<SdkResource>`, as API stubs hold `<APIPage>`. */
export function renderResourceStub(resource: SdkResource): string {
  return [
    "---",
    `title: ${yamlString(resource.title)}`,
    `description: ${yamlString(`${countOf(resource.ops.length, "method")} on the ${resource.title} resource.`)}`,
    `_generated: ${SDK_GENERATED_MARKER}`,
    "---",
    "",
    "{/* Generated by scripts/generate-sdk-reference.ts. Do not edit. */}",
    "",
    `<SdkResource id="${resource.id}" />`,
    "",
  ].join("\n");
}

export function renderSdkMeta(reference: SdkReference): string {
  const meta = {
    title: "SDK Reference",
    icon: "Boxes",
    pages: [
      "index",
      "---Client---",
      ...SDK_GUIDE_PAGES,
      "---Resources---",
      ...reference.resources.map(resource => resource.slug),
    ],
  };
  return `${JSON.stringify(meta, null, 4)}\n`;
}
