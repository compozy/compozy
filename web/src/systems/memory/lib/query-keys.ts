export const memoryKeys = {
  all: ["memory"] as const,
  indexes: () => [...memoryKeys.all, "index"] as const,
  index: (workspaceId: string, slug: string, taskGroupId?: string) =>
    [...memoryKeys.indexes(), workspaceId, slug, taskGroupId ?? null] as const,
  files: () => [...memoryKeys.all, "file"] as const,
  file: (workspaceId: string, slug: string, fileId: string, taskGroupId?: string) =>
    [...memoryKeys.files(), workspaceId, slug, taskGroupId ?? null, fileId] as const,
};
