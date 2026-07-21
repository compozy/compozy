export const specKeys = {
  all: ["spec"] as const,
  workflow: (workspaceId: string, slug: string, taskGroupId?: string) =>
    [...specKeys.all, workspaceId, slug, taskGroupId ?? null] as const,
};
