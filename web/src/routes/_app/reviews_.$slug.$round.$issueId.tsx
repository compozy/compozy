import { useState, type ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

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
        className="text-xs text-accent hover:underline"
        data-testid="review-detail-header-back"
        onClick={() => void navigate({ to: "/reviews" })}
        type="button"
      >
        ← Back to reviews
      </button>
      <span className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
        review issue
      </span>
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
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="review-detail-round-invalid"
          role="alert"
        >
          Invalid review round: {round}
        </p>
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
        <p className="text-sm text-muted-foreground" data-testid="review-detail-loading">
          Loading review issue…
        </p>
      ) : null}
      {issueQuery.isError && !issueQuery.data ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="review-detail-load-error"
          role="alert"
        >
          {apiErrorMessage(
            issueQuery.error,
            `Failed to load review issue ${issueId} for ${slug} round ${round}`
          )}
        </p>
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
