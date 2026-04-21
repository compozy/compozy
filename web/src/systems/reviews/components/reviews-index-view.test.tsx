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

import { ReviewsIndexView, resolveSeverityTone, type ReviewRoundCard } from "@/systems/reviews";
import { resolveReviewStatusTone } from "@/systems/reviews";

interface RenderProps {
  cards?: ReviewRoundCard[];
  isLoading?: boolean;
  isRefetching?: boolean;
  error?: string | null;
}

async function renderIndex(props: RenderProps = {}) {
  const rootRoute = createRootRoute();
  const listRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/",
    component: function ListRoute(): ReactElement {
      return (
        <ReviewsIndexView
          cards={props.cards ?? []}
          error={props.error ?? null}
          isLoading={props.isLoading ?? false}
          isRefetching={props.isRefetching ?? false}
          workspaceName="one"
        />
      );
    },
  });
  const detailRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/reviews/$slug/$round/$issueId",
    component: function DetailStub(): ReactElement {
      return <div data-testid="detail-stub" />;
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([listRoute, detailRoute]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    defaultPreload: false,
  });
  await router.load();
  await act(async () => {
    render(<RouterProvider router={router} />);
    await Promise.resolve();
  });
}

const populatedCard: ReviewRoundCard = {
  slug: "alpha",
  review: {
    workflow_slug: "alpha",
    round_number: 2,
    pr_ref: "PR-42",
    provider: "coderabbit",
    resolved_count: 1,
    unresolved_count: 3,
    updated_at: "2026-01-02T00:00:00Z",
  },
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
  isIssuesLoading: false,
};

describe("ReviewsIndexView", () => {
  it("Should render the loading state", async () => {
    await renderIndex({ isLoading: true });
    expect(screen.getByTestId("reviews-index-loading")).toBeInTheDocument();
  });

  it("Should render the empty state when there are no review rounds", async () => {
    await renderIndex({ cards: [] });
    expect(screen.getByTestId("reviews-index-empty")).toBeInTheDocument();
  });

  it("Should render the error alert when provided", async () => {
    await renderIndex({ error: "workspace stale" });
    expect(screen.getByTestId("reviews-index-error")).toHaveTextContent("workspace stale");
  });

  it("Should render cards and issue rows with detail links", async () => {
    await renderIndex({ cards: [populatedCard] });
    expect(screen.getByTestId("reviews-index-card-alpha")).toBeInTheDocument();
    expect(screen.getByTestId("reviews-index-card-unresolved-alpha")).toHaveTextContent("3");
    expect(screen.getByTestId("reviews-index-card-resolved-alpha")).toHaveTextContent("1");
    const link = screen.getByTestId(
      "reviews-index-issue-link-alpha-issue_001"
    ) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("/reviews/alpha/2/issue_001");
  });

  it("Should render the card issues-loading state when issues are fetching", async () => {
    await renderIndex({
      cards: [
        {
          ...populatedCard,
          issues: [],
          isIssuesLoading: true,
        },
      ],
    });
    expect(screen.getByTestId("reviews-index-card-issues-loading-alpha")).toBeInTheDocument();
  });

  it("Should render the card issues empty state", async () => {
    await renderIndex({
      cards: [
        {
          ...populatedCard,
          issues: [],
          isIssuesLoading: false,
        },
      ],
    });
    expect(screen.getByTestId("reviews-index-card-issues-empty-alpha")).toBeInTheDocument();
  });

  it("Should render the card issues error state", async () => {
    await renderIndex({
      cards: [
        {
          ...populatedCard,
          issues: [],
          isIssuesLoading: false,
          issuesError: "boom",
        },
      ],
    });
    expect(screen.getByTestId("reviews-index-card-issues-error-alpha")).toHaveTextContent("boom");
  });

  it("Should render the refreshing indicator", async () => {
    await renderIndex({ cards: [populatedCard], isRefetching: true });
    expect(screen.getByTestId("reviews-index-refreshing")).toBeInTheDocument();
  });
});

describe("review tone helpers", () => {
  it("Should map severity to tone", () => {
    expect(resolveSeverityTone("critical")).toBe("danger");
    expect(resolveSeverityTone("high")).toBe("danger");
    expect(resolveSeverityTone("medium")).toBe("warning");
    expect(resolveSeverityTone("low")).toBe("info");
    expect(resolveSeverityTone("unknown")).toBe("neutral");
  });

  it("Should map issue status to tone", () => {
    expect(resolveReviewStatusTone("resolved")).toBe("success");
    expect(resolveReviewStatusTone("fixed")).toBe("success");
    expect(resolveReviewStatusTone("dispatched")).toBe("accent");
    expect(resolveReviewStatusTone("invalid")).toBe("neutral");
    expect(resolveReviewStatusTone("open")).toBe("warning");
    expect(resolveReviewStatusTone("pending")).toBe("warning");
    expect(resolveReviewStatusTone("unknown")).toBe("info");
  });
});
