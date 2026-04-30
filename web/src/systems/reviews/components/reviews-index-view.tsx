import type { ReactElement } from "react";

import {
  Alert,
  EmptyState,
  SectionHeading,
  SkeletonRow,
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

import type { ReviewSummary } from "../types";

export interface ReviewRoundCard {
  slug: string;
  review: ReviewSummary;
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
        description={`Review rounds across ${workspaceName}. Open a round to inspect issues and dispatch review fixes.`}
        eyebrow="Across workflows"
        title="Reviews"
      />

      {error ? (
        <Alert data-testid="reviews-index-error" variant="error">
          {error}
        </Alert>
      ) : null}

      {isLoading ? (
        <div className="space-y-2" data-testid="reviews-index-loading">
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}

      {!isLoading && cards.length === 0 && !error ? (
        <EmptyState
          data-testid="reviews-index-empty"
          description="No workflow in this workspace has an active review round yet. Sync a workspace or push a fresh PR review to see rounds here."
          title="No review rounds"
        />
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
  const { slug, review } = card;
  const tone = resolveReviewTone(review);
  const roundLabel = String(review.round_number).padStart(3, "0");
  return (
    <SurfaceCard data-interactive="true" data-testid={`reviews-index-card-${slug}`}>
      <SurfaceCardHeader>
        <div className="min-w-0">
          <SurfaceCardEyebrow>round {roundLabel}</SurfaceCardEyebrow>
          <SurfaceCardTitle>
            <Link
              className="block truncate text-foreground hover:underline"
              data-testid={`reviews-index-round-link-${slug}`}
              params={{ slug, round: String(review.round_number) }}
              title={slug}
              to="/reviews/$slug/$round"
            >
              {slug}
            </Link>
          </SurfaceCardTitle>
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
            label="Issues"
            testId={`reviews-index-card-loaded-${slug}`}
            value={review.resolved_count + review.unresolved_count}
          />
          <Stat
            label="Round"
            testId={`reviews-index-card-round-${slug}`}
            value={review.round_number}
          />
        </div>
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
        <Link
          className="text-xs font-semibold uppercase tracking-[0.12em] text-primary transition-colors hover:text-foreground"
          data-testid={`reviews-index-card-open-${slug}`}
          params={{ slug, round: String(review.round_number) }}
          to="/reviews/$slug/$round"
        >
          Open round →
        </Link>
        <span className="text-xs text-muted-foreground">
          {review.unresolved_count} unresolved / {review.resolved_count} resolved
        </span>
      </SurfaceCardFooter>
    </SurfaceCard>
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
      className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2"
      data-testid={testId}
    >
      <p className="eyebrow text-muted-foreground">{label}</p>
      <p className="mt-1 font-mono text-lg text-foreground tabular-nums">{value}</p>
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
