import { useQuery } from "@tanstack/react-query";

import { getWorkflowBoard, getWorkflowTask } from "../adapters/tasks-api";
import { workflowKeys } from "../lib/query-keys";
import type { TaskBoardPayload, TaskDetailPayload } from "../types";

export function useWorkflowBoard(
  workspaceId: string | null,
  slug: string | null,
  taskGroupId?: string
) {
  return useQuery<TaskBoardPayload>({
    queryKey: workflowKeys.board(workspaceId ?? "none", slug ?? "none", taskGroupId),
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load the task board");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load the task board");
      }
      return getWorkflowBoard({ workspaceId, slug, taskGroupId });
    },
    enabled: Boolean(workspaceId) && Boolean(slug),
  });
}

export function useWorkflowTask(
  workspaceId: string | null,
  slug: string | null,
  taskId: string | null,
  taskGroupId?: string
) {
  return useQuery<TaskDetailPayload>({
    queryKey: workflowKeys.task(
      workspaceId ?? "none",
      slug ?? "none",
      taskId ?? "none",
      taskGroupId
    ),
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load a task");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load a task");
      }
      if (!taskId) {
        throw new Error("task id is required to load a task");
      }
      return getWorkflowTask({ workspaceId, slug, taskId, taskGroupId });
    },
    enabled: Boolean(workspaceId) && Boolean(slug) && Boolean(taskId),
  });
}
