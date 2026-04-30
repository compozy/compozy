import { QueryClientProvider } from "@tanstack/react-query";
import { createMemoryHistory, createRouter, RouterProvider } from "@tanstack/react-router";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { createTestQueryClient, installFetchStub, matchPath } from "@/test/utils";

import { routeTree } from "../routeTree.gen";
import { resetActiveWorkspaceStoreForTests } from "../systems/app-shell";

const workspaceOne = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workflowSummary = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const latestReview = {
  workflow_slug: "alpha",
  round_number: 2,
  pr_ref: "PR-42",
  provider: "coderabbit",
  resolved_count: 1,
  unresolved_count: 3,
  updated_at: "2026-01-02T00:00:00Z",
};

const dashboardPayload = {
  dashboard: {
    workspace: workspaceOne,
    daemon: {
      pid: 1,
      started_at: "2026-01-01T00:00:00Z",
      workspace_count: 1,
      active_run_count: 0,
      http_port: 5555,
      version: "0.0.0",
    },
    health: { ready: true },
    queue: { active: 0, completed: 0, failed: 0, canceled: 0, total: 0 },
    pending_reviews: 1,
    workflows: [
      {
        workflow: workflowSummary,
        active_runs: 0,
        task_total: 0,
        task_completed: 0,
        task_pending: 0,
        review_round_count: 1,
        latest_review: latestReview,
      },
    ],
    active_runs: [],
  },
};

const issuesPayload = {
  issues: [
    {
      id: "issue_004",
      issue_number: 4,
      severity: "medium",
      status: "open",
      source_path: "packages/providers/manifest_test.ts",
      updated_at: "2026-01-02T00:00:00Z",
    },
  ],
};

const reviewRoundPayload = {
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
};

const reviewDetailPayload = {
  review: {
    workspace: workspaceOne,
    workflow: workflowSummary,
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
      title: "Reviewer comment",
      updated_at: "2026-01-02T00:00:00Z",
      markdown: "## Reviewer comment\nadd the missing empty-capabilities test",
    },
    related_runs: [],
  },
};

const reviewRunResponse = {
  run: {
    run_id: "run-review-1",
    workspace_id: "ws-1",
    workflow_slug: "alpha",
    mode: "review",
    presentation_mode: "text",
    started_at: "2026-01-02T00:10:00Z",
    status: "queued",
  },
};

function matchUrl(pattern: string, method: string = "GET") {
  return (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
    const requestMethod =
      input instanceof Request ? input.method.toUpperCase() : (init?.method ?? "GET").toUpperCase();
    return url.includes(pattern) && requestMethod === method.toUpperCase();
  };
}

async function renderApp(initialPath: string) {
  const queryClient = createTestQueryClient();
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [initialPath] }),
    defaultPreload: false,
  });
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  );
  await router.load();
  return { router, queryClient };
}

describe("reviews flow integration", () => {
  let restore: (() => void) | null = null;

  beforeEach(async () => {
    resetActiveWorkspaceStoreForTests();
    window.sessionStorage.clear();
    const { useActiveWorkspaceStore } =
      await import("../systems/app-shell/stores/active-workspace-store");
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: "ws-1" });
  });

  afterEach(() => {
    restore?.();
    restore = null;
    resetActiveWorkspaceStoreForTests();
    vi.clearAllMocks();
  });

  it("Should render compact review round cards without fetching inline issues", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/ui/dashboard"),
        status: 200,
        body: dashboardPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews");
    await screen.findByTestId("reviews-index-view");
    await screen.findByTestId("reviews-index-card-alpha");
    const link = await screen.findByTestId("reviews-index-round-link-alpha");
    expect((link as HTMLAnchorElement).getAttribute("href")).toBe("/reviews/alpha/2");
    expect(stub.calls.some(call => call.url.includes("/api/reviews/alpha/rounds/2/issues"))).toBe(
      false
    );
  });

  it("Should render a review round detail with issue links", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2"),
        status: 200,
        body: reviewRoundPayload,
      },
      {
        matcher: matchPath("/api/reviews/alpha/rounds/2/issues"),
        status: 200,
        body: issuesPayload,
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews/alpha/2");
    await screen.findByTestId("review-round-detail-view");
    const link = await screen.findByTestId("review-round-issue-link-alpha-issue_004");
    expect((link as HTMLAnchorElement).getAttribute("href")).toBe("/reviews/alpha/2/issue_004");
    const roundCall = stub.calls.find(call => call.url.endsWith("/api/reviews/alpha/rounds/2"));
    expect(roundCall?.headers["x-compozy-workspace-id"]).toBe("ws-1");
    const issuesCall = stub.calls.find(call =>
      call.url.endsWith("/api/reviews/alpha/rounds/2/issues")
    );
    expect(issuesCall?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should render the empty-state when no workflow has a latest review", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/ui/dashboard"),
        status: 200,
        body: {
          dashboard: {
            ...dashboardPayload.dashboard,
            pending_reviews: 0,
            workflows: [
              {
                workflow: workflowSummary,
                active_runs: 0,
                task_total: 0,
                task_completed: 0,
                task_pending: 0,
                review_round_count: 0,
              },
            ],
          },
        },
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews");
    await screen.findByTestId("reviews-index-empty");
  });

  it("Should return to workspace selection when the reviews index reports stale workspace context", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/ui/dashboard"),
        status: 412,
        body: {
          code: "workspace_context_stale",
          message: "workspace stale",
          request_id: "r",
        },
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews");
    expect(await screen.findByTestId("workspace-picker-stale")).toBeInTheDocument();
    expect(screen.queryByTestId("reviews-index-error")).not.toBeInTheDocument();
  });

  it("Should render review issue detail and dispatch a review-fix run", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/reviews/alpha/rounds/2/issues/issue_004"),
        status: 200,
        body: reviewDetailPayload,
      },
      {
        matcher: matchUrl("/api/reviews/alpha/rounds/2/runs", "POST"),
        status: 201,
        body: reviewRunResponse,
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews/alpha/2/issue_004");
    await screen.findByTestId("review-detail-view");
    await userEvent.click(screen.getByTestId("review-detail-dispatch-fix"));
    await waitFor(() => {
      expect(screen.getByTestId("review-detail-dispatch-success")).toBeInTheDocument();
    });
    const link = screen.getByTestId("review-detail-dispatch-success-link") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/runs/run-review-1");
    const postedCall = stub.calls.find(call => call.method === "POST");
    expect(postedCall?.body ?? "").toContain('"workspace":"ws-1"');
  });

  it("Should surface a not-found alert for stale or invalid review issues", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
      {
        matcher: matchUrl("/api/reviews/alpha/rounds/2/issues/ghost"),
        status: 404,
        body: { code: "review_issue_not_found", message: "issue missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews/alpha/2/ghost");
    const alert = await screen.findByTestId("review-detail-load-error");
    expect(alert).toHaveTextContent("issue missing");
  });

  it("Should validate the round segment and reject non-numeric rounds", async () => {
    const stub = installFetchStub([
      {
        matcher: matchUrl("/api/workspaces"),
        status: 200,
        body: { workspaces: [workspaceOne] },
      },
    ]);
    restore = stub.restore;
    await renderApp("/reviews/alpha/not-a-number/issue_004");
    const alert = await screen.findByTestId("review-detail-round-invalid");
    expect(alert).toHaveTextContent("not-a-number");
  });
});
