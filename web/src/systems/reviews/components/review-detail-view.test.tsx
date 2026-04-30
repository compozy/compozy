import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import { ReviewDetailView } from "./review-detail-view";
import type { ReviewDetailPayload, ReviewRelatedRun } from "@/systems/reviews";

const workspace = {
  id: "ws-1",
  name: "one",
  root_dir: "/tmp/one",
  filesystem_state: "present",
  read_only: false,
  has_catalog_data: true,
  workflow_count: 1,
  run_count: 0,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
} as const;

const workflow = { id: "wf-1", slug: "alpha", workspace_id: "ws-1" };

const payload: ReviewDetailPayload = {
  workspace,
  workflow,
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
    markdown: "## Reviewer comment\nadd a test",
  },
  related_runs: [
    {
      run_id: "run-review-1",
      workspace_id: "ws-1",
      workflow_slug: "alpha",
      mode: "review",
      presentation_mode: "text",
      started_at: "2026-01-02T00:10:00Z",
      status: "running",
    },
  ],
};

interface RenderProps {
  onDispatch?: () => void;
  isDispatching?: boolean;
  isReadOnly?: boolean;
  dispatchError?: string | null;
  dispatchedRun?: ReviewRelatedRun | null;
}

async function renderDetail(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function DetailRoute(): ReactElement {
      return (
        <ReviewDetailView
          dispatchError={props.dispatchError ?? null}
          dispatchedRun={props.dispatchedRun ?? null}
          isDispatching={props.isDispatching ?? false}
          isReadOnly={props.isReadOnly ?? false}
          isRefreshing={false}
          onDispatchFix={props.onDispatch ?? (() => {})}
          payload={payload}
        />
      );
    },
  });
  const reviewsRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/reviews",
    component: function ReviewsStub(): ReactElement {
      return <div data-testid="reviews-stub" />;
    },
  });
  const roundRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/reviews/$slug/$round",
    component: function RoundStub(): ReactElement {
      return <div data-testid="round-stub" />;
    },
  });
  const runRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/runs/$runId",
    component: function RunStub(): ReactElement {
      return <div data-testid="run-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([detailRoute, reviewsRoute, roundRoute, runRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("ReviewDetailView", () => {
  it("Should render the review document and metadata", async () => {
    await renderDetail();
    expect(screen.getByTestId("review-detail-view")).toBeInTheDocument();
    expect(screen.getByTestId("review-detail-severity")).toHaveTextContent("medium");
    expect(screen.getByTestId("review-detail-status")).toHaveTextContent("open");
    expect(screen.getByTestId("review-detail-round-number")).toHaveTextContent("2");
    expect(screen.getByTestId("review-detail-provider")).toHaveTextContent("coderabbit");
    expect(screen.getByTestId("review-detail-document-body")).toHaveTextContent("add a test");
    const backLink = screen.getByTestId("review-detail-back") as HTMLAnchorElement;
    expect(backLink.getAttribute("href")).toBe("/reviews/alpha/2");
  });

  it("Should invoke the dispatch-fix handler", async () => {
    const onDispatch = vi.fn();
    await renderDetail({ onDispatch });
    await userEvent.click(screen.getByTestId("review-detail-dispatch-fix"));
    expect(onDispatch).toHaveBeenCalledOnce();
  });

  it("Should render the dispatch error alert", async () => {
    await renderDetail({ dispatchError: "conflict" });
    expect(screen.getByTestId("review-detail-dispatch-error")).toHaveTextContent("conflict");
  });

  it("Should render the dispatched-run confirmation with a deep link", async () => {
    await renderDetail({
      dispatchedRun: {
        run_id: "run-review-1",
        workspace_id: "ws-1",
        workflow_slug: "alpha",
        mode: "review",
        presentation_mode: "text",
        started_at: "2026-01-02T00:10:00Z",
        status: "queued",
      },
    });
    const link = screen.getByTestId("review-detail-dispatch-success-link") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/runs/run-review-1");
  });

  it("Should disable the dispatch-fix button while dispatching", async () => {
    await renderDetail({ isDispatching: true });
    const button = screen.getByTestId("review-detail-dispatch-fix") as HTMLButtonElement;
    expect(button.disabled).toBe(true);
    expect(button.textContent ?? "").toContain("Dispatching");
  });

  it("Should disable dispatch-fix when the workspace is read-only", async () => {
    await renderDetail({ isReadOnly: true });
    expect(screen.getByTestId("review-detail-readonly")).toBeInTheDocument();
    expect(screen.getByTestId("review-detail-dispatch-fix")).toBeDisabled();
  });

  it("Should render related runs with run-detail links", async () => {
    await renderDetail();
    const runLink = screen.getByTestId("review-detail-run-link-run-review-1") as HTMLAnchorElement;
    expect(runLink.getAttribute("href")).toBe("/runs/run-review-1");
    expect(
      screen.getByTestId("review-detail-run-status-run-review-1").getAttribute("style")
    ).toContain("var(--tone-accent-bg)");
  });
});
