import { useMemo, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { useDashboard } from "@/systems/dashboard";
import { ReviewsIndexView, type ReviewRoundCard } from "@/systems/reviews";

export const Route = createFileRoute("/_app/reviews")({
  component: ReviewsIndexRoute,
});

function ReviewsIndexRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const dashboardQuery = useDashboard(activeWorkspace.id);

  const reviewableWorkflows = useMemo(() => {
    const workflows = dashboardQuery.data?.workflows ?? [];
    return workflows
      .filter(card => Boolean(card.latest_review))
      .map(card => ({ slug: card.workflow.slug, review: card.latest_review! }));
  }, [dashboardQuery.data?.workflows]);

  const cards: ReviewRoundCard[] = reviewableWorkflows.map(entry => ({
    slug: entry.slug,
    review: entry.review,
  }));

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
