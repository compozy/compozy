import type { RunListParams } from "../types";

export const runKeys = {
  all: ["runs"] as const,
  lists: () => [...runKeys.all, "list"] as const,
  list: (params: RunListParams) =>
    [
      ...runKeys.lists(),
      params.workspaceId ?? "none",
      params.status ?? "all",
      params.mode ?? "all",
      params.limit ?? 0,
    ] as const,
  runs: () => [...runKeys.all, "detail"] as const,
  run: (runId: string) => [...runKeys.runs(), runId, "summary"] as const,
  snapshot: (runId: string) => [...runKeys.runs(), runId, "snapshot"] as const,
  transcript: (runId: string) => [...runKeys.runs(), runId, "transcript"] as const,
};
