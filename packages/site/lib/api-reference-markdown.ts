/**
 * A generated API page as Markdown, for the `llms` output: the page's MDX only passes its
 * operations to `<OpenAPIPage>`, so the Markdown lists them from `lib/api-reference-data.json`
 * (written by `scripts/generate-openapi.ts`, `bun run generate:openapi`).
 */
import data from "@/lib/api-reference-data.json";
import {
  apiOperationKey,
  type ApiFieldFacts,
  type ApiOperationFacts,
} from "@/lib/api-reference-build";
import { apiOperations } from "@/lib/api-reference";

const OPERATIONS = data as Record<string, ApiOperationFacts>;

function fieldLine(field: ApiFieldFacts): string {
  const facts = [field.in, `\`${field.type}\``, field.required ? "required" : ""].filter(Boolean);
  const description = field.description ? `: ${field.description}` : "";
  return `- \`${field.name}\` (${facts.join(", ")})${description}`;
}

export function apiOperationMarkdown(operation: ApiOperationFacts): string {
  const lines = [`## ${operation.summary}`, "", `\`${operation.method} ${operation.path}\``];
  if (operation.parameters.length > 0) {
    lines.push("", "Parameters:", "", ...operation.parameters.map(fieldLine));
  }
  const { body } = operation;
  if (body) {
    const facts = [body.contentType, `\`${body.type}\``, body.required ? "required" : ""];
    lines.push("", `Request body (${facts.filter(Boolean).join(", ")}):`);
    if (body.fields.length > 0) lines.push("", ...body.fields.map(fieldLine));
  }
  return lines.join("\n");
}

/** The editor note fumadocs-openapi writes at the top of every page it generates. */
const GENERATED_COMMENT = /\{\/\*[\s\S]*?\*\/\}\n*/g;

/**
 * The Markdown of a generated API page: its processed text without the generated-file comment,
 * then one section per operation its `<OpenAPIPage>` renders. Any other page is returned as is.
 */
export function expandApiOperations(processed: string, rawSource: string): string {
  const operations = apiOperations(rawSource);
  if (operations.length === 0) return processed;
  const sections = operations.map(({ method, path }) => {
    const operation = OPERATIONS[apiOperationKey(method, path)];
    if (!operation) throw new Error(`${method} ${path} is missing from api-reference-data.json`);
    return apiOperationMarkdown(operation);
  });
  return [processed.replace(GENERATED_COMMENT, "").trim(), ...sections]
    .filter(Boolean)
    .join("\n\n");
}
