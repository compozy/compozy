export const memoryKeys = {
  all: ["memory"] as const,
  indexes: () => [...memoryKeys.all, "index"] as const,
  index: (workspaceId: string, slug: string, packageId?: string) =>
    [...memoryKeys.indexes(), workspaceId, slug, packageId ?? null] as const,
  files: () => [...memoryKeys.all, "file"] as const,
  file: (workspaceId: string, slug: string, fileId: string, packageId?: string) =>
    [...memoryKeys.files(), workspaceId, slug, packageId ?? null, fileId] as const,
};
