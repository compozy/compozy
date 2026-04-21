import type { ReactElement } from "react";

import {
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
  type StatusBadgeTone,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { ReviewIssue, ReviewSummary } from "../types";

export interface ReviewRoundCard {
  slug: string;
  review: ReviewSummary;
  issues: ReviewIssue[];
  isIssuesLoading: boolean;
  issuesError?: string | null;
}

export interface ReviewsIndexViewProps {
  cards: ReviewRoundCard[];
  isLoading: boolean;
  isRefetching: boolean;
  error?: string | null;
  workspaceName: string;
}

export function ReviewsIndexView(props: ReviewsIndexViewProps): ReactElement {
  const { cards, isLoading, isRefetching, error, workspaceName } = props;

  return (
    <div className="space-y-6" data-testid="reviews-index-view">
      <SectionHeading
        description={`Review rounds across ${workspaceName}. Drill into an issue to inspect the patch and dispatch a review fix.`}
        eyebrow="Across workflows"
        title="Reviews"
      />

      {error ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="reviews-index-error"
          role="alert"
        >
          {error}
        </p>
      ) : null}

      {isLoading ? (
        <p className="text-sm text-muted-foreground" data-testid="reviews-index-loading">
          Loading reviews…
        </p>
      ) : null}

      {!isLoading && cards.length === 0 && !error ? (
        <SurfaceCard data-testid="reviews-index-empty">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>Empty</SurfaceCardEyebrow>
              <SurfaceCardTitle>No review rounds</SurfaceCardTitle>
              <SurfaceCardDescription>
                No workflow in this workspace has an active review round yet. Sync a workspace or
                push a fresh PR review to see rounds here.
              </SurfaceCardDescription>
            </div>
          </SurfaceCardHeader>
        </SurfaceCard>
      ) : null}

      {cards.length > 0 ? (
        <div className="space-y-4" data-testid="reviews-index-cards">
          {cards.map(card => (
            <ReviewRoundSection card={card} key={card.slug} />
          ))}
        </div>
      ) : null}

      {isRefetching ? (
        <p className="text-xs text-muted-foreground" data-testid="reviews-index-refreshing">
          refreshing…
        </p>
      ) : null}
    </div>
  );
}

function ReviewRoundSection({ card }: { card: ReviewRoundCard }): ReactElement {
  const { slug, review, issues, isIssuesLoading, issuesError } = card;
  const tone = resolveReviewTone(review);
  const roundLabel = String(review.round_number).padStart(3, "0");
  return (
    <SurfaceCard data-testid={`reviews-index-card-${slug}`}>
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>round {roundLabel}</SurfaceCardEyebrow>
          <SurfaceCardTitle>{slug}</SurfaceCardTitle>
          <SurfaceCardDescription>
            {review.pr_ref ? `PR ${review.pr_ref} · ` : ""}updated{" "}
            {formatTimestamp(review.updated_at)}
          </SurfaceCardDescription>
        </div>
        <StatusBadge data-testid={`reviews-index-card-tone-${slug}`} tone={tone}>
          {review.unresolved_count > 0 ? "open" : "clean"}
        </StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody className="space-y-3">
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Stat
            label="Unresolved"
            testId={`reviews-index-card-unresolved-${slug}`}
            value={review.unresolved_count}
          />
          <Stat
            label="Resolved"
            testId={`reviews-index-card-resolved-${slug}`}
            value={review.resolved_count}
          />
          <Stat
            label="Issues loaded"
            testId={`reviews-index-card-loaded-${slug}`}
            value={issues.length}
          />
          <Stat
            label="Round"
            testId={`reviews-index-card-round-${slug}`}
            value={review.round_number}
          />
        </div>

        {issuesError ? (
          <p
            className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-3 py-2 text-sm text-[color:var(--color-danger)]"
            data-testid={`reviews-index-card-issues-error-${slug}`}
            role="alert"
          >
            {issuesError}
          </p>
        ) : null}

        {isIssuesLoading && issues.length === 0 ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid={`reviews-index-card-issues-loading-${slug}`}
          >
            Loading issues…
          </p>
        ) : null}

        {!isIssuesLoading && issues.length === 0 && !issuesError ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid={`reviews-index-card-issues-empty-${slug}`}
          >
            No issues in this round.
          </p>
        ) : null}

        {issues.length > 0 ? (
          <ul className="space-y-2" data-testid={`reviews-index-card-issues-${slug}`}>
            {issues.map(issue => (
              <ReviewIssueRow
                issue={issue}
                key={issue.id}
                round={review.round_number}
                slug={slug}
              />
            ))}
          </ul>
        ) : null}
      </SurfaceCardBody>
      <SurfaceCardFooter>
        {review.provider ? (
          <span
            className="text-xs text-muted-foreground"
            data-testid={`reviews-index-card-provider-${slug}`}
          >
            via {review.provider}
          </span>
        ) : (
          <span className="text-xs text-muted-foreground" />
        )}
        <span className="text-xs text-muted-foreground">
          {review.unresolved_count} unresolved / {review.resolved_count} resolved
        </span>
      </SurfaceCardFooter>
    </SurfaceCard>
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
  const severityTone = resolveSeverityTone(issue.severity);
  const statusTone = resolveStatusTone(issue.status);
  return (
    <li
      className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
      data-testid={`reviews-index-issue-${slug}-${issue.id}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="eyebrow text-muted-foreground">issue #{issue.issue_number}</p>
          <Link
            className="truncate text-sm font-medium text-foreground hover:underline"
            data-testid={`reviews-index-issue-link-${slug}-${issue.id}`}
            params={{ slug, round: String(round), issueId: issue.id }}
            to="/reviews/$slug/$round/$issueId"
          >
            {issue.source_path}
          </Link>
          <p className="text-xs text-muted-foreground">
            updated {formatTimestamp(issue.updated_at)}
          </p>
        </div>
        <div className="flex flex-col items-end gap-1 sm:flex-row sm:items-center">
          <StatusBadge
            data-testid={`reviews-index-issue-severity-${slug}-${issue.id}`}
            tone={severityTone}
          >
            {issue.severity}
          </StatusBadge>
          <StatusBadge
            data-testid={`reviews-index-issue-status-${slug}-${issue.id}`}
            tone={statusTone}
          >
            {issue.status}
          </StatusBadge>
        </div>
      </div>
    </li>
  );
}

function Stat({
  label,
  value,
  testId,
}: {
  label: string;
  value: number;
  testId: string;
}): ReactElement {
  return (
    <div
      className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
      data-testid={testId}
    >
      <p className="eyebrow text-muted-foreground">{label}</p>
      <p className="mt-1 font-display text-lg tracking-[-0.02em] text-foreground">{value}</p>
    </div>
  );
}

function resolveReviewTone(review: ReviewSummary): StatusBadgeTone {
  if (review.unresolved_count === 0) {
    return "success";
  }
  if (review.unresolved_count >= 5) {
    return "danger";
  }
  return "warning";
}

export function resolveSeverityTone(severity: string): StatusBadgeTone {
  const normalized = severity.trim().toLowerCase();
  switch (normalized) {
    case "critical":
    case "high":
      return "danger";
    case "medium":
      return "warning";
    case "low":
      return "info";
    default:
      return "neutral";
  }
}

export function resolveStatusTone(status: string): StatusBadgeTone {
  const normalized = status.trim().toLowerCase();
  switch (normalized) {
    case "resolved":
    case "fixed":
      return "success";
    case "in_progress":
    case "dispatched":
      return "accent";
    case "invalid":
      return "neutral";
    case "open":
    case "pending":
      return "warning";
    default:
      return "info";
  }
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
