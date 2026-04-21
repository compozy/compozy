export const specKeys = {
  all: ["spec"] as const,
  workflow: (workspaceId: string, slug: string) => [...specKeys.all, workspaceId, slug] as const,
};
