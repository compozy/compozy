export const reviewKeys = {
  all: ["reviews"] as const,
  summaries: () => [...reviewKeys.all, "summary"] as const,
  summary: (workspaceId: string, slug: string) =>
    [...reviewKeys.summaries(), workspaceId, slug] as const,
  rounds: () => [...reviewKeys.all, "round"] as const,
  round: (workspaceId: string, slug: string, round: number) =>
    [...reviewKeys.rounds(), workspaceId, slug, round] as const,
  issues: (workspaceId: string, slug: string, round: number) =>
    [...reviewKeys.round(workspaceId, slug, round), "issues"] as const,
  issue: (workspaceId: string, slug: string, round: number, issueId: string) =>
    [...reviewKeys.issues(workspaceId, slug, round), issueId] as const,
};
