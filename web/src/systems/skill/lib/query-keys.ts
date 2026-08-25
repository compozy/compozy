export const skillKeys = {
  all: ["skills"] as const,
  listScope: (workspace: string) => [...skillKeys.all, "list", workspace] as const,
  list: (workspace: string, profile?: string) =>
    [...skillKeys.listScope(workspace), profile ?? ""] as const,
  detail: (name: string, workspace: string, profile?: string) =>
    [...skillKeys.all, "detail", name, workspace, profile ?? ""] as const,
  content: (name: string, workspace: string, profile?: string) =>
    [...skillKeys.all, "content", name, workspace, profile ?? ""] as const,
  shadows: (name: string, workspace: string, profile?: string) =>
    [...skillKeys.all, "shadows", name, workspace, profile ?? ""] as const,
};
