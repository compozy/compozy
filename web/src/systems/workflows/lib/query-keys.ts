export const workflowKeys = {
  all: ["workflows"] as const,
  lists: () => [...workflowKeys.all, "list"] as const,
  list: (workspaceId: string) => [...workflowKeys.lists(), workspaceId] as const,
  workflows: () => [...workflowKeys.all, "workflow"] as const,
  board: (workspaceId: string, slug: string, taskGroupId?: string) =>
    [...workflowKeys.workflows(), workspaceId, slug, taskGroupId ?? null, "board"] as const,
  tasks: (workspaceId: string, slug: string, taskGroupId?: string) =>
    [...workflowKeys.workflows(), workspaceId, slug, taskGroupId ?? null, "task"] as const,
  task: (workspaceId: string, slug: string, taskId: string, taskGroupId?: string) =>
    [...workflowKeys.tasks(workspaceId, slug, taskGroupId), taskId] as const,
};
