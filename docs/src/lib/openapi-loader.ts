import { readFileSync } from "node:fs";
import { readFile } from "node:fs/promises";
import path from "node:path";

let cachedDocument: unknown;

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
  const swaggerPath = path.join(process.cwd(), "swagger.json");
  const raw = await readFile(swaggerPath, "utf8");
  return parseDocument(raw);
}

function readAndProcessSync(): unknown {
  const swaggerPath = path.join(process.cwd(), "swagger.json");
  const raw = readFileSync(swaggerPath, "utf8");
  return parseDocument(raw);
}

function parseDocument(raw: string): unknown {
  const document = JSON.parse(raw) as Record<string, unknown>;
  ensureSchemaTitles(document);
  return document;
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
