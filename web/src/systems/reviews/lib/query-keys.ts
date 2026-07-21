export const reviewKeys = {
  all: ["reviews"] as const,
  summaries: () => [...reviewKeys.all, "summary"] as const,
  summary: (workspaceId: string, slug: string, taskGroupId?: string) =>
    [...reviewKeys.summaries(), workspaceId, slug, taskGroupId ?? null] as const,
  rounds: () => [...reviewKeys.all, "round"] as const,
  round: (workspaceId: string, slug: string, round: number, taskGroupId?: string) =>
    [...reviewKeys.rounds(), workspaceId, slug, taskGroupId ?? null, round] as const,
  issues: (workspaceId: string, slug: string, round: number, taskGroupId?: string) =>
    [...reviewKeys.round(workspaceId, slug, round, taskGroupId), "issues"] as const,
  issue: (
    workspaceId: string,
    slug: string,
    round: number,
    issueId: string,
    taskGroupId?: string
  ) => [...reviewKeys.issues(workspaceId, slug, round, taskGroupId), issueId] as const,
};
