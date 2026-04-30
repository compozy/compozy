import { useState, type ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { Alert, SkeletonRow } from "@compozy/ui";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import {
  ReviewDetailView,
  useReviewIssue,
  useStartReviewRun,
  type ReviewRelatedRun,
} from "@/systems/reviews";

export const Route = createFileRoute("/_app/reviews_/$slug/$round/$issueId")({
  component: ReviewIssueDetailRoute,
  parseParams: params => ({
    slug: params.slug,
    round: params.round,
    issueId: params.issueId,
  }),
});

function ReviewIssueDetailRoute(): ReactElement {
  const { slug, round, issueId } = useParams({
    from: "/_app/reviews_/$slug/$round/$issueId",
  });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const parsedRound = parseRound(round);
  const issueQuery = useReviewIssue(
    activeWorkspace.id,
    slug,
    Number.isFinite(parsedRound) ? parsedRound : null,
    issueId
  );
  const startReviewRun = useStartReviewRun();
  const [dispatchedRun, setDispatchedRun] = useState<ReviewRelatedRun | null>(null);

  const header = (
    <div className="flex w-full items-center justify-between gap-3">
      <button
        className="text-xs font-medium text-primary transition-colors hover:text-foreground"
        data-testid="review-detail-header-back"
        onClick={() => void navigate({ to: "/reviews" })}
        type="button"
      >
        ← Back to reviews
      </button>
      <span className="eyebrow text-muted-foreground">review issue</span>
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
        <Alert data-testid="review-detail-round-invalid" variant="error">
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
      {issueQuery.isLoading && !issueQuery.data ? (
        <div className="space-y-3" data-testid="review-detail-loading">
          <p className="sr-only">Loading review issue…</p>
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {issueQuery.isError && !issueQuery.data ? (
        <Alert data-testid="review-detail-load-error" variant="error">
          {apiErrorMessage(
            issueQuery.error,
            `Failed to load review issue ${issueId} for ${slug} round ${round}`
          )}
        </Alert>
      ) : null}
      {issueQuery.data ? (
        <ReviewDetailView
          dispatchError={
            startReviewRun.isError
              ? apiErrorMessage(startReviewRun.error, "Failed to dispatch review fix")
              : null
          }
          dispatchedRun={dispatchedRun}
          isDispatching={startReviewRun.isPending}
          isReadOnly={activeWorkspace.read_only}
          isRefreshing={issueQuery.isRefetching}
          onDispatchFix={() => {
            startReviewRun.mutate(
              {
                workspaceId: activeWorkspace.id,
                slug,
                round: parsedRound,
              },
              {
                onSuccess: run => setDispatchedRun(run),
              }
            );
          }}
          payload={issueQuery.data}
        />
      ) : null}
    </AppShellLayout>
  );
}

function parseRound(raw: string): number {
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : Number.NaN;
}
