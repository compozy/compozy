/**
 * The facts the `llms` Markdown of a generated API page lists for each operation, read from the
 * OpenAPI document. Pure: `scripts/generate-openapi.ts` writes the result to
 * `lib/api-reference-data.json`, so no route reads the 2.7 MB contract at request time.
 */
import { schemaType } from "@/lib/sdk-reference-build";

type Schema = NonNullable<Parameters<typeof schemaType>[0]> & {
  properties?: Record<string, Schema & { description?: string }>;
  required?: string[];
};

type Parameter = {
  name: string;
  in: string;
  required?: boolean;
  description?: string;
  schema?: Schema;
};

type Operation = {
  summary?: string;
  parameters?: Parameter[];
  requestBody?: {
    required?: boolean;
    content?: Record<string, { schema?: Schema }>;
  };
};

export type ApiOpenAPIDocument = {
  paths?: Record<string, Record<string, unknown>>;
  components?: { schemas?: Record<string, Schema> };
};

export interface ApiFieldFacts {
  name: string;
  type: string;
  required: boolean;
  description?: string;
  /** Where a parameter goes: `path`, `query`, `header` or `cookie`. Body fields have none. */
  in?: string;
}

export interface ApiBodyFacts {
  contentType: string;
  /** The body schema's name, or its short type when it is inline. */
  type: string;
  required: boolean;
  /** Top-level properties of the body object, in the contract's order. */
  fields: ApiFieldFacts[];
}

export interface ApiOperationFacts {
  method: string;
  path: string;
  summary: string;
  parameters: ApiFieldFacts[];
  body: ApiBodyFacts | null;
}

/** `GET /api/sessions`: how the data file and the page's `operations` prop name one operation. */
export function apiOperationKey(method: string, path: string): string {
  return `${method.toUpperCase()} ${path}`;
}

const HTTP_METHODS = new Set(["get", "post", "put", "patch", "delete"]);

function withDescription<T extends object>(facts: T, description: string | undefined): T {
  return description ? { ...facts, description } : facts;
}

function resolve(document: ApiOpenAPIDocument, schema: Schema | undefined): Schema | undefined {
  if (!schema?.$ref) return schema;
  return document.components?.schemas?.[schema.$ref.split("/").at(-1) ?? ""];
}

function bodyOf(document: ApiOpenAPIDocument, operation: Operation): ApiBodyFacts | null {
  const [contentType, media] = Object.entries(operation.requestBody?.content ?? {})[0] ?? [];
  if (!contentType) return null;
  const schema = resolve(document, media?.schema);
  const required = new Set(schema?.required ?? []);
  return {
    contentType,
    type: schemaType(media?.schema),
    required: Boolean(operation.requestBody?.required),
    fields: Object.entries(schema?.properties ?? {}).map(([name, field]) =>
      withDescription(
        { name, type: schemaType(field), required: required.has(name) },
        field.description
      )
    ),
  };
}

/** One entry per operation of the document, keyed by {@link apiOperationKey}. */
export function buildApiOperationFacts(
  document: ApiOpenAPIDocument
): Record<string, ApiOperationFacts> {
  const facts: Record<string, ApiOperationFacts> = {};
  for (const [path, item] of Object.entries(document.paths ?? {})) {
    for (const [method, value] of Object.entries(item)) {
      if (!HTTP_METHODS.has(method) || typeof value !== "object" || value === null) continue;
      const operation = value as Operation;
      facts[apiOperationKey(method, path)] = {
        method: method.toUpperCase(),
        path,
        summary: operation.summary ?? "",
        parameters: (operation.parameters ?? []).map(param =>
          withDescription(
            {
              name: param.name,
              in: param.in,
              type: schemaType(param.schema),
              required: Boolean(param.required),
            },
            param.description
          )
        ),
        body: bodyOf(document, operation),
      };
    }
  }
  return facts;
}
