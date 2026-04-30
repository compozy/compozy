import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import { act, render, screen } from "@testing-library/react";
import type { RenderResult } from "@testing-library/react";
import { createContext, useContext, type ReactElement } from "react";
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

interface RenderProps {
  issues?: ReviewIssue[];
  issuesError?: string | null;
  isIssuesLoading?: boolean;
  isRefreshing?: boolean;
}

const ReviewRoundDetailTestContext = createContext<RenderProps | null>(null);

async function renderRoundDetail(props: RenderProps = {}) {
  let currentProps = props;
  const rootRoute = createRootRoute();
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function DetailRoute(): ReactElement {
      const value = useContext(ReviewRoundDetailTestContext);
      if (!value) {
        throw new Error("expected review round detail test context");
      }
      return (
        <ReviewRoundDetailView
          issues={value.issues ?? issues}
          issuesError={value.issuesError ?? null}
          isIssuesLoading={value.isIssuesLoading ?? false}
          isRefreshing={value.isRefreshing ?? false}
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
  let renderResult: RenderResult | null = null;
  await act(async () => {
    renderResult = render(
      <ReviewRoundDetailTestContext.Provider value={currentProps}>
        <RouterProvider router={router} />
      </ReviewRoundDetailTestContext.Provider>
    );
    await Promise.resolve();
  });
  return {
    rerender(nextProps: typeof props) {
      if (renderResult === null) {
        throw new Error("expected render result");
      }
      currentProps = nextProps;
      act(() => {
        renderResult!.rerender(
          <ReviewRoundDetailTestContext.Provider value={currentProps}>
            <RouterProvider router={router} />
          </ReviewRoundDetailTestContext.Provider>
        );
      });
    },
  };
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
    const view = await renderRoundDetail({ issues: [], isIssuesLoading: true, isRefreshing: true });
    expect(screen.getByTestId("review-round-loading")).toBeInTheDocument();
    expect(screen.getByTestId("review-round-refreshing")).toBeInTheDocument();

    view.rerender({ issues: [], issuesError: "boom" });
    expect(screen.getByTestId("review-round-issues-error")).toHaveTextContent("boom");

    view.rerender({ issues: [] });
    expect(screen.getByTestId("review-round-empty")).toBeInTheDocument();
  });
});
