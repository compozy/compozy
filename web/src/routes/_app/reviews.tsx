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

  const cards: ReviewRoundCard[] = useMemo(() => {
    const workflows = dashboardQuery.data?.workflows ?? [];
    const rounds: ReviewRoundCard[] = [];
    for (const card of workflows) {
      if (card.latest_review) {
        rounds.push({ slug: card.workflow.slug, review: card.latest_review });
      }
      // Task Group rounds route through the parent initiative slug plus task_group_id,
      // matching the task-group-aware review routes.
      for (const taskGroup of card.workflow.task_groups ?? []) {
        if (taskGroup.latest_review) {
          rounds.push({
            slug: card.workflow.slug,
            review: taskGroup.latest_review,
            taskGroupId: taskGroup.task_group_id,
          });
        }
      }
    }
    return rounds;
  }, [dashboardQuery.data?.workflows]);

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
