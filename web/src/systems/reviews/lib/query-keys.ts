export const reviewKeys = {
  all: ["reviews"] as const,
  summaries: () => [...reviewKeys.all, "summary"] as const,
  summary: (workspaceId: string, slug: string, packageId?: string) =>
    [...reviewKeys.summaries(), workspaceId, slug, packageId ?? null] as const,
  rounds: () => [...reviewKeys.all, "round"] as const,
  round: (workspaceId: string, slug: string, round: number, packageId?: string) =>
    [...reviewKeys.rounds(), workspaceId, slug, packageId ?? null, round] as const,
  issues: (workspaceId: string, slug: string, round: number, packageId?: string) =>
    [...reviewKeys.round(workspaceId, slug, round, packageId), "issues"] as const,
  issue: (workspaceId: string, slug: string, round: number, issueId: string, packageId?: string) =>
    [...reviewKeys.issues(workspaceId, slug, round, packageId), issueId] as const,
};
