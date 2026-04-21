import { http, HttpResponse, type HttpHandler } from "msw";

import type { WorkflowSpecDocument } from "../types";
import { workflowSpecFixture } from "./fixtures";

export interface SpecHandlerOptions {
  spec?: WorkflowSpecDocument;
}

export function createSpecHandlers(options: SpecHandlerOptions = {}): HttpHandler[] {
  const spec = options.spec ?? workflowSpecFixture;

  return [http.get("/api/tasks/:slug/spec", () => HttpResponse.json({ spec }))];
}

export const handlers = createSpecHandlers();
