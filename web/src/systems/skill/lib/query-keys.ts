export const skillKeys = {
  all: ["skills"] as const,
  list: (workspace: string) => [...skillKeys.all, "list", workspace] as const,
  detail: (name: string, workspace: string) =>
    [...skillKeys.all, "detail", name, workspace] as const,
  content: (name: string, workspace: string) =>
    [...skillKeys.all, "content", name, workspace] as const,
  shadows: (name: string, workspace: string) =>
    [...skillKeys.all, "shadows", name, workspace] as const,
};
