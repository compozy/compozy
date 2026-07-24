import { daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type { TaskBoardPayload, TaskDetailPayload } from "../types";

function workspaceHeader(workspaceId: string) {
  return { header: { [ACTIVE_WORKSPACE_HEADER]: workspaceId } } as const;
}

function taskGroupQuery(taskGroupId: string | undefined) {
  return taskGroupId ? { query: { task_group_id: taskGroupId } } : {};
}

export interface WorkflowBoardParams {
  workspaceId: string;
  slug: string;
  taskGroupId?: string;
}

export async function getWorkflowBoard(params: WorkflowBoardParams): Promise<TaskBoardPayload> {
  const { data, error, response } = await daemonApiClient.GET("/api/tasks/{slug}/board", {
    params: {
      path: { slug: params.slug },
      ...taskGroupQuery(params.taskGroupId),
      ...workspaceHeader(params.workspaceId),
    },
  });
  const payload = requireData(
    data,
    response,
    `Failed to load task board for ${params.slug}`,
    error
  );
  return payload.board;
}

export interface WorkflowTaskParams {
  workspaceId: string;
  slug: string;
  taskId: string;
  taskGroupId?: string;
}

export async function getWorkflowTask(params: WorkflowTaskParams): Promise<TaskDetailPayload> {
  const { data, error, response } = await daemonApiClient.GET("/api/tasks/{slug}/items/{task_id}", {
    params: {
      path: { slug: params.slug, task_id: params.taskId },
      ...taskGroupQuery(params.taskGroupId),
      ...workspaceHeader(params.workspaceId),
    },
  });
  const payload = requireData(
    data,
    response,
    `Failed to load task ${params.taskId} for ${params.slug}`,
    error
  );
  return payload.task;
}
