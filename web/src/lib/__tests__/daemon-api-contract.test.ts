import { describe, expectTypeOf, it } from "vitest";

import type {
  DaemonOperationPath,
  DaemonOperationRequestBody,
  DaemonOperationResponse,
} from "@/lib/api-contract";

type ResolveWorkspaceBody = DaemonOperationRequestBody<"resolveWorkspace">;
type DashboardResponse = DaemonOperationResponse<"getDashboard", 200>;
type WorkflowTaskPath = DaemonOperationPath<"getWorkflowTask">;
type ReviewIssuesResponse = DaemonOperationResponse<"listReviewIssues", 200>;
type StartWorkflowRunBody = DaemonOperationRequestBody<"startWorkflowRun">;
type RunSnapshotResponse = DaemonOperationResponse<"getRunSnapshot", 200>;
type CancelRunBody = DaemonOperationRequestBody<"cancelRun">;
type DaemonMetricsResponse = DaemonOperationResponse<"getDaemonMetrics", 200>;

describe("daemon browser openapi contract", () => {
  it("keeps generated daemon-web operation types aligned with the checked-in contract", () => {
    expectTypeOf<ResolveWorkspaceBody["path"]>().toEqualTypeOf<string>();

    expectTypeOf<DashboardResponse["dashboard"]["daemon"]["status"]>().toEqualTypeOf<
      "starting" | "ready" | "degraded" | "stopped"
    >();
    expectTypeOf<
      DashboardResponse["dashboard"]["workflows"][number]["slug"]
    >().toEqualTypeOf<string>();
    expectTypeOf<
      DashboardResponse["dashboard"]["pending_reviews"][number]["status"]
    >().toEqualTypeOf<"open" | "in_progress" | "resolved">();

    expectTypeOf<WorkflowTaskPath["slug"]>().toEqualTypeOf<string>();
    expectTypeOf<WorkflowTaskPath["task_id"]>().toEqualTypeOf<string>();

    expectTypeOf<ReviewIssuesResponse["issues"][number]["severity"]>().toEqualTypeOf<
      "low" | "medium" | "high" | "critical"
    >();

    expectTypeOf<StartWorkflowRunBody["provider"]>().toEqualTypeOf<string | undefined>();
    expectTypeOf<CancelRunBody["reason"]>().toEqualTypeOf<string | undefined>();

    expectTypeOf<RunSnapshotResponse["snapshot"]["timeline"][number]["type"]>().toEqualTypeOf<
      "snapshot" | "event" | "heartbeat" | "overflow"
    >();
    expectTypeOf<RunSnapshotResponse["snapshot"]["run"]["status"]>().toEqualTypeOf<
      "queued" | "starting" | "running" | "succeeded" | "failed" | "canceled"
    >();

    expectTypeOf<DaemonMetricsResponse>().toEqualTypeOf<string>();
  });
});
