import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { describe, expect, it } from "vitest";

import type { ReviewIssue, ReviewRound } from "@/systems/reviews";

import { ReviewRoundDetailView } from "./review-round-detail-view";

const round: ReviewRound = {
  id: "round-2",
  workflow_slug: "alpha",
  round_number: 2,
  provider: "coderabbit",
  pr_ref: "PR-42",
  resolved_count: 1,
  unresolved_count: 3,
  updated_at: "2026-01-02T00:00:00Z",
};

const issues: ReviewIssue[] = [
  {
    id: "issue_001",
    issue_number: 1,
    severity: "medium",
    status: "open",
    source_path:
      "reviews-002/issue_001_with_a_very_long_identifier_that_must_truncate_before_badges.md",
    updated_at: "2026-01-02T00:00:00Z",
  },
];

async function renderRoundDetail(
  props: {
    issues?: ReviewIssue[];
    issuesError?: string | null;
    isIssuesLoading?: boolean;
    isRefreshing?: boolean;
  } = {}
) {
  const rootRoute = createRootRoute();
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function DetailRoute(): ReactElement {
      return (
        <ReviewRoundDetailView
          issues={props.issues ?? issues}
          issuesError={props.issuesError ?? null}
          isIssuesLoading={props.isIssuesLoading ?? false}
          isRefreshing={props.isRefreshing ?? false}
          round={round}
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
  const issueRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/reviews/$slug/$round/$issueId",
    component: function IssueStub(): ReactElement {
      return <div data-testid="issue-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([detailRoute, reviewsRoute, issueRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

describe("ReviewRoundDetailView", () => {
  it("Should render review issues with issue-detail links", async () => {
    await renderRoundDetail();
    expect(screen.getByTestId("review-round-detail-view")).toBeInTheDocument();
    expect(screen.getByTestId("review-round-status")).toHaveTextContent("open");
    const link = screen.getByTestId("review-round-issue-link-alpha-issue_001") as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/reviews/alpha/2/issue_001");
    expect(link.className).toContain("truncate");
  });

  it("Should render loading, error, empty, and refreshing states", async () => {
    await renderRoundDetail({ issues: [], isIssuesLoading: true, isRefreshing: true });
    expect(screen.getByTestId("review-round-loading")).toBeInTheDocument();
    expect(screen.getByTestId("review-round-refreshing")).toBeInTheDocument();

    await renderRoundDetail({ issues: [], issuesError: "boom" });
    expect(screen.getByTestId("review-round-issues-error")).toHaveTextContent("boom");

    await renderRoundDetail({ issues: [] });
    expect(screen.getByTestId("review-round-empty")).toBeInTheDocument();
  });
});
