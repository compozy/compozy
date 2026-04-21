import type { components } from "@/generated/compozy-openapi";

export type Run = components["schemas"]["Run"];
export type RunSnapshot = components["schemas"]["RunSnapshotPayload"];
export type RunJobState = components["schemas"]["RunJobState"];
export type RunJobSummary = components["schemas"]["RunJobSummary"];
export type RunTranscriptMessage = components["schemas"]["RunTranscriptMessage"];
export type RunShutdownState = components["schemas"]["RunShutdownState"];
export type RunUsage = components["schemas"]["Usage"];
export type TaskRunRequestBody = components["schemas"]["TaskRunRequest"];

export type RunListStatusFilter = "active" | "completed" | "failed" | "canceled" | "all";
export type RunListModeFilter = "task" | "review" | "exec" | "all";

export interface RunListParams {
  workspaceId: string | null;
  status?: RunListStatusFilter;
  mode?: RunListModeFilter;
  limit?: number;
}
