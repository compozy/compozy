import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { Alert, SkeletonRow } from "@compozy/ui";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { ReviewRoundDetailView, useReviewIssues, useReviewRound } from "@/systems/reviews";

export const Route = createFileRoute("/_app/reviews_/$slug/$round")({
  component: ReviewRoundDetailRoute,
  parseParams: params => ({
    slug: params.slug,
    round: params.round,
  }),
});

function ReviewRoundDetailRoute(): ReactElement {
  const { slug, round } = useParams({
    from: "/_app/reviews_/$slug/$round",
  });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const parsedRound = parseRound(round);
  const roundQuery = useReviewRound(
    activeWorkspace.id,
    slug,
    Number.isFinite(parsedRound) ? parsedRound : null
  );
  const issuesQuery = useReviewIssues(
    activeWorkspace.id,
    slug,
    Number.isFinite(parsedRound) ? parsedRound : null
  );

  const header = (
    <div className="flex w-full items-center justify-between gap-3">
      <button
        className="text-xs font-medium text-primary transition-colors hover:text-foreground"
        data-testid="review-round-header-back"
        onClick={() => void navigate({ to: "/reviews" })}
        type="button"
      >
        Back to reviews
      </button>
      <span className="eyebrow text-muted-foreground">review round</span>
    </div>
  );

  if (!Number.isFinite(parsedRound)) {
    return (
      <AppShellLayout
        activeWorkspace={activeWorkspace}
        onSwitchWorkspace={onSwitchWorkspace}
        workspaces={workspaces}
        header={header}
      >
        <Alert data-testid="review-round-invalid" variant="error">
          Invalid review round: {round}
        </Alert>
      </AppShellLayout>
    );
  }

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={header}
    >
      {roundQuery.isLoading && !roundQuery.data ? (
        <div className="space-y-3" data-testid="review-round-route-loading">
          <p className="sr-only">Loading review round...</p>
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {roundQuery.isError && !roundQuery.data ? (
        <Alert data-testid="review-round-load-error" variant="error">
          {apiErrorMessage(roundQuery.error, `Failed to load review round ${round} for ${slug}`)}
        </Alert>
      ) : null}
      {roundQuery.data ? (
        <ReviewRoundDetailView
          issues={issuesQuery.data ?? []}
          issuesError={
            issuesQuery.isError
              ? apiErrorMessage(issuesQuery.error, `Failed to load issues for ${slug}`)
              : null
          }
          isIssuesLoading={issuesQuery.isLoading && !issuesQuery.data}
          isRefreshing={roundQuery.isRefetching || issuesQuery.isRefetching}
          round={roundQuery.data}
        />
      ) : null}
    </AppShellLayout>
  );
}

function parseRound(raw: string): number {
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : Number.NaN;
}
