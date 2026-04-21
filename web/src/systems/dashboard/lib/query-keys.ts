export const dashboardKeys = {
  all: ["dashboard"] as const,
  byWorkspace: (workspaceId: string) => [...dashboardKeys.all, workspaceId] as const,
};
