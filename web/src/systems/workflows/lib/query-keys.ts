export const workflowKeys = {
  all: ["workflows"] as const,
  lists: () => [...workflowKeys.all, "list"] as const,
  list: (workspaceId: string) => [...workflowKeys.lists(), workspaceId] as const,
  workflows: () => [...workflowKeys.all, "workflow"] as const,
  board: (workspaceId: string, slug: string) =>
    [...workflowKeys.workflows(), workspaceId, slug, "board"] as const,
  tasks: (workspaceId: string, slug: string) =>
    [...workflowKeys.workflows(), workspaceId, slug, "task"] as const,
  task: (workspaceId: string, slug: string, taskId: string) =>
    [...workflowKeys.tasks(workspaceId, slug), taskId] as const,
};
