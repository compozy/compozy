import { readFileSync } from "node:fs";
import path from "node:path";

let cachedDocument: unknown;
const SWAGGER_PATH = path.join(process.cwd(), "swagger.json");

export async function loadOpenAPIDocument(): Promise<unknown> {
  if (cachedDocument) {
    return cachedDocument;
  }
  const document = await readAndProcess();
  cachedDocument = document;
  return document;
}

export function loadOpenAPIDocumentSync(): unknown {
  if (cachedDocument) {
    return cachedDocument;
  }
  const document = readAndProcessSync();
  cachedDocument = document;
  return document;
}

async function readAndProcess(): Promise<unknown> {
  const file = Bun.file(SWAGGER_PATH);
  if (!(await file.exists())) {
    throw new Error(`OpenAPI loader: swagger.json not found at ${SWAGGER_PATH}`);
  }
  const raw = await file.text();
  return parseDocument(raw);
}

function readAndProcessSync(): unknown {
  try {
    const raw = readFileSync(SWAGGER_PATH, "utf8");
    return parseDocument(raw);
  } catch (err) {
    throw new Error(`OpenAPI loader: failed to read ${SWAGGER_PATH}: ${(err as Error).message}`);
  }
}

function parseDocument(raw: string): unknown {
  try {
    const document = JSON.parse(raw) as Record<string, unknown>;
    ensureSchemaTitles(document);
    return document;
  } catch (err) {
    throw new Error(`OpenAPI loader: invalid JSON in swagger.json: ${(err as Error).message}`);
  }
}

function ensureSchemaTitles(document: Record<string, unknown>): void {
  attachTitles(document.definitions as Record<string, unknown> | undefined);
  const components = document.components as { schemas?: Record<string, unknown> } | undefined;
  if (components?.schemas) {
    attachTitles(components.schemas);
  }
}

function attachTitles(record: Record<string, unknown> | undefined): void {
  if (!record) {
    return;
  }
  for (const [name, schema] of Object.entries(record)) {
    if (!schema || typeof schema !== "object") {
      continue;
    }
    const typed = schema as { title?: string };
    if (typed.title == null) {
      typed.title = name;
    }
  }
}
