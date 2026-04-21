export const memoryKeys = {
  all: ["memory"] as const,
  indexes: () => [...memoryKeys.all, "index"] as const,
  index: (workspaceId: string, slug: string) =>
    [...memoryKeys.indexes(), workspaceId, slug] as const,
  files: () => [...memoryKeys.all, "file"] as const,
  file: (workspaceId: string, slug: string, fileId: string) =>
    [...memoryKeys.files(), workspaceId, slug, fileId] as const,
};
