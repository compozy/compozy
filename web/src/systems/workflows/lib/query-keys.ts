export const workflowKeys = {
  all: ["workflows"] as const,
  lists: () => [...workflowKeys.all, "list"] as const,
  list: (workspaceId: string) => [...workflowKeys.lists(), workspaceId] as const,
  workflows: () => [...workflowKeys.all, "workflow"] as const,
  board: (workspaceId: string, slug: string, packageId?: string) =>
    [...workflowKeys.workflows(), workspaceId, slug, packageId ?? null, "board"] as const,
  tasks: (workspaceId: string, slug: string, packageId?: string) =>
    [...workflowKeys.workflows(), workspaceId, slug, packageId ?? null, "task"] as const,
  task: (workspaceId: string, slug: string, taskId: string, packageId?: string) =>
    [...workflowKeys.tasks(workspaceId, slug, packageId), taskId] as const,
};
