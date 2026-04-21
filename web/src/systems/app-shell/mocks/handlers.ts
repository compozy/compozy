import { http, HttpResponse, type HttpHandler } from "msw";

import type { Workspace } from "../types";
import { buildWorkspacesFixture, workspaceFixture } from "./fixtures";

export interface AppShellHandlerOptions {
  workspaces?: Workspace[];
  resolvedWorkspace?: Workspace;
}

export function createAppShellHandlers(options: AppShellHandlerOptions = {}): HttpHandler[] {
  const workspaces = options.workspaces ?? buildWorkspacesFixture();
  const resolvedWorkspace = options.resolvedWorkspace ?? workspaces[0] ?? workspaceFixture;

  return [
    http.get("/api/workspaces", () => HttpResponse.json({ workspaces })),
    http.post("/api/workspaces/resolve", () =>
      HttpResponse.json({ workspace: resolvedWorkspace }, { status: 201 })
    ),
  ];
}

export const handlers = createAppShellHandlers();
