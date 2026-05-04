import { apiErrorMessage, daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type { ArchiveResult, SyncResult, WorkflowSummary } from "../types";

function workspaceParams(workspaceId: string) {
  return { header: { [ACTIVE_WORKSPACE_HEADER]: workspaceId } } as const;
}

export async function listWorkflows(workspaceId: string): Promise<WorkflowSummary[]> {
  const { data, error, response } = await daemonApiClient.GET("/api/tasks", {
    params: workspaceParams(workspaceId),
  });
  const payload = requireData(data, response, "Failed to load workflows", error);
  return payload.workflows ?? [];
}

export interface SyncWorkflowParams {
  workspaceId: string;
  workflowSlug?: string;
  path?: string;
}

export async function syncWorkflow(params: SyncWorkflowParams): Promise<SyncResult> {
  const { data, error, response } = await daemonApiClient.POST("/api/sync", {
    params: workspaceParams(params.workspaceId),
    body: {
      ...(params.workflowSlug ? { workflow_slug: params.workflowSlug } : {}),
      ...(params.path ? { path: params.path } : {}),
      workspace: params.workspaceId,
    },
  });
  if (!data) {
    throw new Error(apiErrorMessage(error, "Failed to run sync"));
  }
  return requireData(data, response, "Failed to run sync", error);
}

export interface ArchiveWorkflowParams {
  workspaceId: string;
  slug: string;
  force?: boolean;
}

export async function archiveWorkflow(params: ArchiveWorkflowParams): Promise<ArchiveResult> {
  const { data, error, response } = await daemonApiClient.POST("/api/tasks/{slug}/archive", {
    params: {
      path: { slug: params.slug },
      header: { [ACTIVE_WORKSPACE_HEADER]: params.workspaceId },
    },
    body: {
      workspace: params.workspaceId,
      ...(params.force ? { force: true } : {}),
    },
  });
  return requireData(data, response, "Failed to archive workflow", error);
}
