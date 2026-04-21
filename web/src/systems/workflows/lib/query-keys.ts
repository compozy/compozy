export const workflowKeys = {
  all: ["workflows"] as const,
  lists: () => [...workflowKeys.all, "list"] as const,
  list: (workspaceId: string) => [...workflowKeys.lists(), workspaceId] as const,
};
