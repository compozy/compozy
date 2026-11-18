/** The SDK reference data: guides written from the client READMEs, methods from the OpenAPI document and SDK samples. */

export type SdkLang = "ts" | "go";

export interface SdkField {
  name: string;
  type: string;
  required: boolean;
  description: string;
  in?: string;
}

export interface SdkVariant {
  signature: string;
  fields: SdkField[];
  example: string;
}

export interface SdkSection {
  title: string;
  text: string;
  ts: SdkVariant | null;
  go: SdkVariant | null;
}

export interface SdkGuide {
  slug: string;
  title: string;
  description: string;
  sections: SdkSection[];
}

export interface SdkMethod {
  id: string;
  summary: string;
  method: string;
  path: string;
  kind: string;
  app: boolean;
  params: SdkField[];
  body: string;
  returns: string;
}

export interface SdkResource {
  id: string;
  slug: string;
  title: string;
  ops: SdkMethod[];
}

export interface SdkData {
  install: Record<SdkLang, string>;
  run: Record<SdkLang, string>;
  stats: { operations: number; resources: number; app: number; streams: number };
  guides: SdkGuide[];
  resources: SdkResource[];
}

export type SdkSamples = Record<string, { typescript?: string; go?: string } | undefined>;

export interface SdkBundle {
  data: SdkData;
  samples: SdkSamples;
}

export const LANGS: { id: SdkLang; label: string; module: string }[] = [
  { id: "ts", label: "TypeScript", module: 'import { Compozy } from "@compozy/client";' },
  { id: "go", label: "Go", module: 'import "github.com/compozy/compozy/sdk/go/client"' },
];

/** Both files load on demand: the snapshot beside the draft, and the samples the repo already ships. */
export async function loadSdk(): Promise<SdkBundle> {
  const [data, samples] = await Promise.all([
    import("./sdk.json"),
    import("../../../../openapi/sdk-samples.json"),
  ]);
  return {
    data: data.default as unknown as SdkData,
    samples: samples.default.operations as unknown as SdkSamples,
  };
}

export function isSdkSlug(slug: string) {
  return slug === "sdk" || slug.startsWith("sdk/");
}

function pascal(id: string) {
  return id.charAt(0).toUpperCase() + id.slice(1);
}

export function methodName(method: SdkMethod, lang: SdkLang) {
  return lang === "go" ? pascal(method.id) : method.id;
}

function optionKeys(method: SdkMethod) {
  const keys = [];
  if (method.params.some(param => param.in === "path")) keys.push("path");
  if (method.params.some(param => param.in === "query")) keys.push("query");
  if (method.body) keys.push("body");
  return keys;
}

/** The call shape for one method, as each client exposes it. */
export function signatureOf(resource: SdkResource, method: SdkMethod, lang: SdkLang) {
  if (lang === "go") {
    const args = ["ctx"];
    if (method.params.length > 0) args.push(`${pascal(method.id)}Params`);
    if (method.body) args.push(method.body);
    return `c.${pascal(method.id)}(${args.join(", ")})`;
  }
  const keys = optionKeys(method);
  const options = keys.length > 0 ? `{ ${keys.join(", ")} }` : "";
  if (method.kind === "sse") {
    return `compozy.events("${method.id}"${options ? `, ${options}` : ""})`;
  }
  if (method.kind === "prompt") return "compozy.prompt(input)";
  return `compozy.${resource.id}.${method.id}(${options})`;
}

export function returnsOf(method: SdkMethod, lang: SdkLang) {
  if (method.kind === "sse") return lang === "go" ? "*Stream[T]" : "AsyncIterable<StreamEvent>";
  if (method.kind === "prompt") return lang === "go" ? "*PromptStream" : "Promise<PromptReply>";
  if (!method.returns) return lang === "go" ? "error" : "Promise<void>";
  return lang === "go" ? `*${method.returns}` : `Promise<${method.returns}>`;
}
