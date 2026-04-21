import { http, HttpResponse, type HttpHandler } from "msw";

import type { MarkdownDocument, WorkflowMemoryIndex } from "../types";
import { workflowMemoryDocumentFixture, workflowMemoryIndexFixture } from "./fixtures";

export interface MemoryHandlerOptions {
  index?: WorkflowMemoryIndex;
  document?: MarkdownDocument;
}

export function createMemoryHandlers(options: MemoryHandlerOptions = {}): HttpHandler[] {
  const index = options.index ?? workflowMemoryIndexFixture;
  const document = options.document ?? workflowMemoryDocumentFixture;

  return [
    http.get("/api/tasks/:slug/memory", () => HttpResponse.json({ memory: index })),
    http.get("/api/tasks/:slug/memory/files/:file_id", () => HttpResponse.json({ document })),
  ];
}

export const handlers = createMemoryHandlers();
