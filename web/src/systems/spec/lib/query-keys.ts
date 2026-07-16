export const specKeys = {
  all: ["spec"] as const,
  workflow: (workspaceId: string, slug: string, packageId?: string) =>
    [...specKeys.all, workspaceId, slug, packageId ?? null] as const,
};
