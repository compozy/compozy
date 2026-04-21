import { daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type { MarkdownDocument, WorkflowMemoryIndex } from "../types";

function workspaceHeader(workspaceId: string) {
  return { header: { [ACTIVE_WORKSPACE_HEADER]: workspaceId } } as const;
}

export interface WorkflowMemoryParams {
  workspaceId: string;
  slug: string;
}

export async function getWorkflowMemoryIndex(
  params: WorkflowMemoryParams
): Promise<WorkflowMemoryIndex> {
  const { data, error, response } = await daemonApiClient.GET("/api/tasks/{slug}/memory", {
    params: {
      path: { slug: params.slug },
      ...workspaceHeader(params.workspaceId),
    },
  });
  const payload = requireData(
    data,
    response,
    `Failed to load memory index for ${params.slug}`,
    error
  );
  return payload.memory;
}

export interface WorkflowMemoryFileParams {
  workspaceId: string;
  slug: string;
  fileId: string;
}

export async function getWorkflowMemoryFile(
  params: WorkflowMemoryFileParams
): Promise<MarkdownDocument> {
  const { data, error, response } = await daemonApiClient.GET(
    "/api/tasks/{slug}/memory/files/{file_id}",
    {
      params: {
        path: { slug: params.slug, file_id: params.fileId },
        ...workspaceHeader(params.workspaceId),
      },
    }
  );
  const payload = requireData(
    data,
    response,
    `Failed to load memory file ${params.fileId} for ${params.slug}`,
    error
  );
  return payload.document;
}
