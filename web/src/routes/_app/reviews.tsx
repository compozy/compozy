import { useMemo, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";
import { useQueries, type QueryKey } from "@tanstack/react-query";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { useDashboard } from "@/systems/dashboard";
import {
  ReviewsIndexView,
  listReviewIssues,
  reviewKeys,
  type ReviewIssue,
  type ReviewRoundCard,
} from "@/systems/reviews";

export const Route = createFileRoute("/_app/reviews")({
  component: ReviewsIndexRoute,
});

interface ReviewIssuesQueryResult {
  data: ReviewIssue[] | undefined;
  isLoading: boolean;
  isError: boolean;
  error: unknown;
}

function ReviewsIndexRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const dashboardQuery = useDashboard(activeWorkspace.id);

  const reviewableWorkflows = useMemo(() => {
    const workflows = dashboardQuery.data?.workflows ?? [];
    return workflows
      .filter(card => Boolean(card.latest_review))
      .map(card => ({ slug: card.workflow.slug, review: card.latest_review! }));
  }, [dashboardQuery.data?.workflows]);

  const issueQueries = useQueries({
    queries: reviewableWorkflows.map(entry => ({
      queryKey: reviewKeys.issues(
        activeWorkspace.id,
        entry.slug,
        entry.review.round_number
      ) as QueryKey,
      queryFn: () =>
        listReviewIssues({
          workspaceId: activeWorkspace.id,
          slug: entry.slug,
          round: entry.review.round_number,
        }),
      enabled: Boolean(activeWorkspace.id),
    })),
  });

  const cards: ReviewRoundCard[] = reviewableWorkflows.map((entry, index) => {
    const query = (issueQueries[index] ?? {
      data: undefined,
      isLoading: false,
      isError: false,
      error: undefined,
    }) as ReviewIssuesQueryResult;
    return {
      slug: entry.slug,
      review: entry.review,
      issues: query.data ?? [],
      isIssuesLoading: query.isLoading,
      issuesError: query.isError
        ? apiErrorMessage(query.error, `Failed to load issues for ${entry.slug}`)
        : null,
    };
  });

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
    >
      <ReviewsIndexView
        cards={cards}
        error={
          dashboardQuery.isError
            ? apiErrorMessage(dashboardQuery.error, "Failed to load reviews")
            : null
        }
        isLoading={dashboardQuery.isLoading && !dashboardQuery.data}
        isRefetching={dashboardQuery.isRefetching}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
