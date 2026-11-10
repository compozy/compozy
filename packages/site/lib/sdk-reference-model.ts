/**
 * The SDK reference model and the call shapes each client exposes, ported from
 * `web/src/drafts/docs/sdk.ts`. Pure: no data loading, so client components can import it.
 */

/** A count with its noun, singular for one: "1 method", "98 methods". */
export function countOf(count: number, noun: string, plural = `${noun}s`): string {
  return `${count} ${count === 1 ? noun : plural}`;
}

export type SdkLang = "ts" | "go";
export type SdkKind = "json" | "sse" | "websocket" | "prompt";

export interface SdkField {
  name: string;
  type: string;
  required: boolean;
  description: string;
  in?: "path" | "query" | "header";
  /** The name each client takes this option under; `null` where that client does not take it. */
  client: Record<SdkLang, string | null>;
}

/** How the TypeScript client reaches an operation, read from `@compozy/client`'s generated types. */
export interface SdkTsCall {
  /** Top-level request options (`path`, `query`, `headers`, `body`) the method accepts. */
  options: string[];
  /** Request body type, as exported by `@compozy/client`. */
  body: string;
  /** Resolved type of the successful response body; `void` when there is none. */
  returns: string;
}

/** How the Go client reaches an operation, read from `sdk/go/client`'s method signatures. */
export interface SdkGoCall {
  name: string;
  /** Argument types after `ctx`, in order (`ListTasksParams`, `CreateTaskRequest`). */
  args: string[];
  /** First result type (`*TaskListResponse`), or `error` when the method returns only an error. */
  returns: string;
}

export interface SdkMethod {
  id: string;
  summary: string;
  method: string;
  path: string;
  kind: SdkKind;
  /** App tokens may call this operation (`app_access: allowed` in the SDK samples). */
  app: boolean;
  params: SdkField[];
  ts: SdkTsCall | null;
  go: SdkGoCall | null;
  samples: Record<SdkLang, string>;
}

export interface SdkResource {
  id: string;
  slug: string;
  title: string;
  ops: SdkMethod[];
}

export interface SdkStats {
  operations: number;
  resources: number;
  app: number;
  streams: number;
}

export interface SdkReference {
  stats: SdkStats;
  resources: SdkResource[];
}

export const SDK_LANGS: readonly { id: SdkLang; label: string; module: string }[] = [
  { id: "ts", label: "TypeScript", module: 'import { Compozy } from "@compozy/client";' },
  { id: "go", label: "Go", module: 'import "github.com/compozy/compozy/sdk/go/client"' },
];

/** What a Go `SdkSymbol` marked `none` says: the symbol exists in TypeScript only. */
export const SDK_NO_GO_EQUIVALENT = "TypeScript only. The Go client has no equivalent.";

export function isSdkLang(value: unknown): value is SdkLang {
  return value === "ts" || value === "go";
}

/** `cmdPalette` → `cmd-palette`, the slug of the resource page. */
export function kebabCase(id: string): string {
  return id.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}

/** The name a reader looks for in each client. */
export function methodName(method: SdkMethod, lang: SdkLang): string {
  if (lang === "go") return method.go?.name ?? method.id;
  return method.kind === "prompt" ? "prompt" : method.id;
}

function tsOptions(method: SdkMethod): string {
  const keys = method.ts?.options ?? [];
  return keys.length > 0 ? `{ ${keys.join(", ")} }` : "";
}

/** The call shape for one method, as each client exposes it; `null` when the client has none. */
export function signatureOf(resource: SdkResource, method: SdkMethod, lang: SdkLang) {
  if (lang === "go") {
    if (!method.go) return null;
    return `c.${method.go.name}(${["ctx", ...method.go.args].join(", ")})`;
  }
  if (method.kind === "prompt") return "compozy.prompt(input)";
  if (!method.ts) return null;
  if (method.kind === "sse") return `compozy.events("${method.id}", ${tsOptions(method) || "{}"})`;
  return `compozy.${resource.id}.${method.id}(${tsOptions(method)})`;
}

export function returnsOf(method: SdkMethod, lang: SdkLang) {
  if (lang === "go") {
    if (method.kind === "prompt") return "*PromptStream";
    return method.go?.returns ?? null;
  }
  if (method.kind === "prompt") return "Promise<PromptReply>";
  if (method.kind === "sse") return `AsyncIterable<StreamEvent<SseFrame<"${method.id}">>>`;
  if (!method.ts) return null;
  return `Promise<${method.ts.returns}>`;
}

/** One option as a reader of one client sees it. */
export interface SdkOption {
  field: SdkField;
  name: string;
}

/**
 * The options one client takes for a method, under that client's names: Go reads them from the
 * method's `…Params` struct, TypeScript from its `path`, `query` and `headers` (never headers for
 * `compozy.events()`), prompts from `PromptInput`. A WebSocket route has no client method, so its
 * route parameters keep their HTTP names.
 */
export function optionsOf(method: SdkMethod, lang: SdkLang): SdkOption[] {
  if (method.kind === "websocket") return method.params.map(field => ({ field, name: field.name }));
  return method.params.flatMap(field => {
    const name = field.client[lang];
    return name === null ? [] : [{ field, name }];
  });
}

/** The request body type each client takes, or an empty string. */
export function bodyOf(method: SdkMethod, lang: SdkLang): string {
  if (lang === "go") {
    const last = method.go?.args.at(-1) ?? "";
    return last.endsWith("Params") ? "" : last;
  }
  return method.ts?.body ?? "";
}
