import { daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type { WorkflowSpecDocument } from "../types";

function taskGroupQuery(taskGroupId: string | undefined) {
  return taskGroupId ? { query: { task_group_id: taskGroupId } } : {};
}

export interface WorkflowSpecParams {
  workspaceId: string;
  slug: string;
  taskGroupId?: string;
}

export async function getWorkflowSpec(params: WorkflowSpecParams): Promise<WorkflowSpecDocument> {
  const { data, error, response } = await daemonApiClient.GET("/api/tasks/{slug}/spec", {
    params: {
      path: { slug: params.slug },
      header: { [ACTIVE_WORKSPACE_HEADER]: params.workspaceId },
      ...taskGroupQuery(params.taskGroupId),
    },
  });
  const payload = requireData(
    data,
    response,
    `Failed to load workflow spec for ${params.slug}`,
    error
  );
  return payload.spec;
}
