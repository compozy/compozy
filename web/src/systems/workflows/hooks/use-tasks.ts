import { useQuery } from "@tanstack/react-query";

import { getWorkflowBoard, getWorkflowTask } from "../adapters/tasks-api";
import { workflowKeys } from "../lib/query-keys";
import type { TaskBoardPayload, TaskDetailPayload } from "../types";

export function useWorkflowBoard(workspaceId: string | null, slug: string | null) {
  return useQuery<TaskBoardPayload>({
    queryKey: workflowKeys.board(workspaceId ?? "none", slug ?? "none"),
    queryFn: () => {
      if (!workspaceId) {
        throw new Error("active workspace is required to load the task board");
      }
      if (!slug) {
        throw new Error("workflow slug is required to load the task board");
      }
      return getWorkflowBoard({ workspaceId, slug });
    },
    enabled: Boolean(workspaceId) && Boolean(slug),
  });
}

export function useWorkflowTask(
  workspaceId: string | null,
  slug: string | null,
  taskId: string | null
) {
  return useQuery<TaskDetailPayload>({
    queryKey: workflowKeys.task(workspaceId ?? "none", slug ?? "none", taskId ?? "none"),
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
      return getWorkflowTask({ workspaceId, slug, taskId });
    },
    enabled: Boolean(workspaceId) && Boolean(slug) && Boolean(taskId),
  });
}
