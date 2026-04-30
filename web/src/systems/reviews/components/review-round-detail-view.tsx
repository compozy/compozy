import type { ReactElement } from "react";

import {
  Alert,
  EmptyState,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { ReviewIssue, ReviewRound } from "../types";

import { resolveSeverityTone, resolveStatusTone } from "./reviews-index-view";

export interface ReviewRoundDetailViewProps {
  round: ReviewRound;
  issues: ReviewIssue[];
  issuesError?: string | null;
  isIssuesLoading: boolean;
  isRefreshing: boolean;
}

export function ReviewRoundDetailView(props: ReviewRoundDetailViewProps): ReactElement {
  const { round, issues, issuesError, isIssuesLoading, isRefreshing } = props;
  const roundLabel = String(round.round_number).padStart(3, "0");
  const tone = round.unresolved_count > 0 ? "warning" : "success";

  return (
    <div className="space-y-6" data-testid="review-round-detail-view">
      <SectionHeading
        description={
          <span>
            <Link
              className="underline-offset-4 hover:underline"
              data-testid="review-round-back"
              to="/reviews"
            >
              Back to reviews
            </Link>
            {" · "}
            {round.pr_ref ? `PR ${round.pr_ref} · ` : ""}
            updated {formatTimestamp(round.updated_at)}
          </span>
        }
        eyebrow={`Review round · ${roundLabel}`}
        title={
          <span className="flex min-w-0 max-w-full flex-wrap items-center gap-3">
            <span className="min-w-0 truncate">{round.workflow_slug}</span>
            <StatusBadge className="shrink-0" data-testid="review-round-status" tone={tone}>
              {round.unresolved_count > 0 ? "open" : "clean"}
            </StatusBadge>
          </span>
        }
      />

      {issuesError ? (
        <Alert data-testid="review-round-issues-error" variant="error">
          {issuesError}
        </Alert>
      ) : null}

      <SurfaceCard data-testid="review-round-issues-card">
        <SurfaceCardHeader>
          <div className="min-w-0">
            <SurfaceCardEyebrow>Issues</SurfaceCardEyebrow>
            <SurfaceCardTitle>Round issue inventory</SurfaceCardTitle>
            <SurfaceCardDescription>
              {round.unresolved_count} unresolved / {round.resolved_count} resolved
            </SurfaceCardDescription>
          </div>
          <StatusBadge tone="info">{issues.length}</StatusBadge>
        </SurfaceCardHeader>
        <SurfaceCardBody>
          {isIssuesLoading && issues.length === 0 ? (
            <p className="text-xs text-muted-foreground" data-testid="review-round-loading">
              loading issues…
            </p>
          ) : null}
          {!isIssuesLoading && issues.length === 0 && !issuesError ? (
            <EmptyState
              className="py-5"
              data-testid="review-round-empty"
              title="No issues in this round"
            />
          ) : null}
          {issues.length > 0 ? (
            <ul className="space-y-2" data-testid="review-round-issues">
              {issues.map(issue => (
                <ReviewIssueRow
                  issue={issue}
                  key={issue.id}
                  round={round.round_number}
                  slug={round.workflow_slug}
                />
              ))}
            </ul>
          ) : null}
          {isRefreshing ? (
            <p className="mt-3 text-xs text-muted-foreground" data-testid="review-round-refreshing">
              refreshing…
            </p>
          ) : null}
        </SurfaceCardBody>
      </SurfaceCard>
    </div>
  );
}

function ReviewIssueRow({
  issue,
  round,
  slug,
}: {
  issue: ReviewIssue;
  round: number;
  slug: string;
}): ReactElement {
  return (
    <li
      className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover"
      data-testid={`review-round-issue-${slug}-${issue.id}`}
    >
      <div className="min-w-0 space-y-1">
        <p className="eyebrow text-muted-foreground">issue #{issue.issue_number}</p>
        <Link
          className="block truncate text-sm font-medium text-foreground hover:underline"
          data-testid={`review-round-issue-link-${slug}-${issue.id}`}
          params={{ slug, round: String(round), issueId: issue.id }}
          title={issue.source_path}
          to="/reviews/$slug/$round/$issueId"
        >
          {issue.source_path}
        </Link>
        <p className="truncate text-xs text-muted-foreground">
          updated {formatTimestamp(issue.updated_at)}
        </p>
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1 sm:flex-row sm:items-center">
        <StatusBadge
          data-testid={`review-round-issue-severity-${slug}-${issue.id}`}
          tone={resolveSeverityTone(issue.severity)}
        >
          {issue.severity}
        </StatusBadge>
        <StatusBadge
          data-testid={`review-round-issue-status-${slug}-${issue.id}`}
          tone={resolveStatusTone(issue.status)}
        >
          {issue.status}
        </StatusBadge>
      </div>
    </li>
  );
}

function formatTimestamp(raw: string | undefined): string {
  if (!raw) {
    return "unknown";
  }
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
