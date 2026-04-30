import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import {
  getLatestReview,
  getReviewIssue,
  getReviewRound,
  listReviewIssues,
  startReviewRun,
} from "./reviews-api";

describe("reviews api adapter", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should GET the latest review summary with the workspace header", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha"),
        status: 200,
        body: {
          review: {
            workflow_slug: "alpha",
            round_number: 2,
            pr_ref: "PR-42",
            provider: "coderabbit",
            resolved_count: 1,
            unresolved_count: 3,
            updated_at: "2026-01-02T00:00:00Z",
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getLatestReview({ workspaceId: "ws-1", slug: "alpha" });
    expect(result.round_number).toBe(2);
    expect(result.unresolved_count).toBe(3);
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should GET one review round with the workspace header", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2"),
        status: 200,
        body: {
          round: {
            id: "round-2",
            workflow_slug: "alpha",
            round_number: 2,
            pr_ref: "PR-42",
            provider: "coderabbit",
            resolved_count: 1,
            unresolved_count: 3,
            updated_at: "2026-01-02T00:00:00Z",
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getReviewRound({ workspaceId: "ws-1", slug: "alpha", round: 2 });
    expect(result.round_number).toBe(2);
    expect(result.workflow_slug).toBe("alpha");
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should GET review issues for one round", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2/issues"),
        status: 200,
        body: {
          issues: [
            {
              id: "issue_001",
              issue_number: 1,
              severity: "medium",
              status: "open",
              source_path: "packages/x/y.ts",
              updated_at: "2026-01-02T00:00:00Z",
            },
          ],
        },
      },
    ]);
    restore = stub.restore;
    const result = await listReviewIssues({ workspaceId: "ws-1", slug: "alpha", round: 2 });
    expect(result).toHaveLength(1);
    expect(result[0]?.id).toBe("issue_001");
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should surface transport errors from the issues endpoint", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2/issues"),
        status: 412,
        body: {
          code: "workspace_context_stale",
          message: "workspace stale",
          request_id: "r",
        },
      },
    ]);
    restore = stub.restore;
    await expect(
      listReviewIssues({ workspaceId: "ws-1", slug: "alpha", round: 2 })
    ).rejects.toThrow(/workspace stale/);
  });

  it("Should GET the review issue detail", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2/issues/issue_004"),
        status: 200,
        body: {
          review: {
            workspace: {
              id: "ws-1",
              name: "one",
              root_dir: "/tmp/one",
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
            workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
            round: {
              id: "round-2",
              pr_ref: "PR-42",
              provider: "coderabbit",
              resolved_count: 1,
              round_number: 2,
              unresolved_count: 3,
              updated_at: "2026-01-02T00:00:00Z",
              workflow_slug: "alpha",
            },
            issue: {
              id: "issue_004",
              issue_number: 4,
              severity: "medium",
              status: "open",
              updated_at: "2026-01-02T00:00:00Z",
            },
            document: {
              id: "review-doc",
              kind: "review",
              title: "Review body",
              updated_at: "2026-01-02T00:00:00Z",
              markdown: "## Review details",
            },
            related_runs: [],
          },
        },
      },
    ]);
    restore = stub.restore;
    const detail = await getReviewIssue({
      workspaceId: "ws-1",
      slug: "alpha",
      round: 2,
      issueId: "issue_004",
    });
    expect(detail.issue.id).toBe("issue_004");
    expect(detail.document.markdown).toContain("Review details");
  });

  it("Should POST a review-fix run and thread the workspace into the body", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2/runs", "POST"),
        status: 201,
        body: {
          run: {
            run_id: "run-review-1",
            workspace_id: "ws-1",
            workflow_slug: "alpha",
            mode: "review",
            presentation_mode: "text",
            started_at: "2026-01-02T00:10:00Z",
            status: "queued",
          },
        },
      },
    ]);
    restore = stub.restore;
    const run = await startReviewRun({
      workspaceId: "ws-1",
      slug: "alpha",
      round: 2,
      body: { presentation_mode: "text" },
    });
    expect(run.run_id).toBe("run-review-1");
    const postedCall = stub.calls.find(call => call.method === "POST");
    expect(postedCall?.body ?? "").toContain('"workspace":"ws-1"');
    expect(postedCall?.body ?? "").toContain('"presentation_mode":"text"');
  });

  it("Should surface transport errors from the review-fix dispatch endpoint", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2/runs", "POST"),
        status: 409,
        body: { code: "run_conflict", message: "already dispatched", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(startReviewRun({ workspaceId: "ws-1", slug: "alpha", round: 2 })).rejects.toThrow(
      /already dispatched/
    );
  });
});
