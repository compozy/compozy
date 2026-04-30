import type { ReviewDetailPayload, ReviewIssue, ReviewRound, ReviewSummary, Run } from "../types";
import { workspaceFixture } from "@/systems/app-shell/mocks";
import { workflowAlphaFixture } from "@/systems/workflows/mocks";

export const latestReviewFixture: ReviewSummary = {
  workflow_slug: workflowAlphaFixture.slug,
  round_number: 2,
  pr_ref: "PR-42",
  provider: "coderabbit",
  resolved_count: 1,
  unresolved_count: 3,
  updated_at: "2026-04-20T02:00:00Z",
};

export const reviewRoundFixture: ReviewRound = {
  id: "round-2",
  pr_ref: "PR-42",
  provider: "coderabbit",
  resolved_count: 1,
  round_number: 2,
  unresolved_count: 3,
  updated_at: "2026-04-20T02:00:00Z",
  workflow_slug: workflowAlphaFixture.slug,
};

export const reviewIssuesFixture: ReviewIssue[] = [
  {
    id: "issue_004",
    issue_number: 4,
    severity: "medium",
    status: "open",
    source_path: "internal/api/core/handlers.go",
    updated_at: "2026-04-20T02:00:00Z",
  },
];

export const reviewDetailFixture: ReviewDetailPayload = {
  workspace: workspaceFixture,
  workflow: {
    id: workflowAlphaFixture.id,
    slug: workflowAlphaFixture.slug,
    workspace_id: workflowAlphaFixture.workspace_id,
  },
  round: reviewRoundFixture,
  issue: {
    id: "issue_004",
    issue_number: 4,
    severity: "medium",
    status: "open",
    updated_at: "2026-04-20T02:00:00Z",
  },
  document: {
    id: "review-doc",
    kind: "review",
    title: "Reviewer comment",
    updated_at: "2026-04-20T02:00:00Z",
    markdown: "## Reviewer comment\nAdd the missing route-state contract test.",
  },
  related_runs: [
    {
      run_id: "run-review-1",
      workspace_id: workspaceFixture.id,
      workflow_slug: workflowAlphaFixture.slug,
      mode: "review",
      presentation_mode: "text",
      started_at: "2026-04-20T02:10:00Z",
      status: "running",
    },
  ],
};

export const reviewDispatchedRunFixture: Run = {
  run_id: "run-review-dispatched",
  workspace_id: workspaceFixture.id,
  workflow_slug: workflowAlphaFixture.slug,
  mode: "review",
  presentation_mode: "text",
  started_at: "2026-04-20T02:12:00Z",
  status: "queued",
};
