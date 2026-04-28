import { apiErrorMessage, daemonApiClient, requireData } from "@/lib/api-client";

import type { Workspace, WorkspaceSyncResult } from "../types";

export async function listWorkspaces(): Promise<Workspace[]> {
  const { data, error, response } = await daemonApiClient.GET("/api/workspaces");
  const payload = requireData(data, response, "Failed to load workspaces", error);
  return payload.workspaces ?? [];
}

export interface ResolveWorkspaceParams {
  path: string;
}

export async function resolveWorkspace(params: ResolveWorkspaceParams): Promise<Workspace> {
  const { data, error, response } = await daemonApiClient.POST("/api/workspaces/resolve", {
    body: { path: params.path },
  });
  if (!data) {
    throw new Error(apiErrorMessage(error, "Failed to resolve workspace"));
  }
  return requireData(data, response, "Failed to resolve workspace", error).workspace;
}

export async function syncWorkspaces(): Promise<WorkspaceSyncResult> {
  const { data, error, response } = await daemonApiClient.POST("/api/workspaces/sync");
  return requireData(data, response, "Failed to sync workspaces", error);
}
