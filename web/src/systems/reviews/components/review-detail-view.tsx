import type { ReactElement } from "react";

import { Wrench } from "lucide-react";

import {
  Alert,
  Button,
  EmptyState,
  Markdown,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import { resolveStatusTone as resolveRunStatusTone } from "@/systems/runs";

import type { ReviewDetailPayload, Run } from "../types";

import { resolveSeverityTone, resolveStatusTone } from "./reviews-index-view";

export interface ReviewDetailViewProps {
  payload: ReviewDetailPayload;
  isRefreshing: boolean;
  onDispatchFix: () => void;
  isDispatching: boolean;
  isReadOnly?: boolean;
  dispatchError?: string | null;
  dispatchedRun?: Run | null;
}

export function ReviewDetailView(props: ReviewDetailViewProps): ReactElement {
  const {
    payload,
    isRefreshing,
    onDispatchFix,
    isDispatching,
    isReadOnly = false,
    dispatchError,
    dispatchedRun,
  } = props;
  const { workflow, round, issue, document } = payload;
  const severityTone = resolveSeverityTone(issue.severity);
  const statusTone = resolveStatusTone(issue.status);
  const markdown = document.markdown?.trim() ?? "";

  return (
    <div className="space-y-6" data-testid="review-detail-view">
      <SectionHeading
        actions={
          <div className="flex items-center gap-2">
            <Button
              data-testid="review-detail-dispatch-fix"
              disabled={isDispatching || isReadOnly}
              icon={<Wrench className="size-4" />}
              loading={isDispatching}
              onClick={onDispatchFix}
              size="sm"
            >
              {isDispatching ? "Dispatching…" : "Dispatch fix"}
            </Button>
          </div>
        }
        description={
          <span>
            <Link
              className="underline-offset-4 hover:underline"
              data-testid="review-detail-back"
              to="/reviews"
            >
              Back to reviews
            </Link>
            {" · "}
            {workflow.slug} · round {String(round.round_number).padStart(3, "0")} · updated{" "}
            {formatTimestamp(issue.updated_at)}
          </span>
        }
        eyebrow={`Review issue · ${issue.id}`}
        title={
          <span className="flex flex-wrap items-center gap-3">
            <span>{document.title}</span>
            <StatusBadge data-testid="review-detail-severity" tone={severityTone}>
              {issue.severity}
            </StatusBadge>
            <StatusBadge data-testid="review-detail-status" tone={statusTone}>
              {issue.status}
            </StatusBadge>
          </span>
        }
      />

      {dispatchError ? (
        <Alert data-testid="review-detail-dispatch-error" variant="error">
          {dispatchError}
        </Alert>
      ) : null}
      {isReadOnly ? (
        <Alert data-testid="review-detail-readonly" variant="warning">
          Workspace path missing. Review fix runs are disabled until the path is restored.
        </Alert>
      ) : null}

      {dispatchedRun ? (
        <Alert data-testid="review-detail-dispatch-success" variant="success">
          Dispatched run{" "}
          <Link
            className="font-mono underline"
            data-testid="review-detail-dispatch-success-link"
            params={{ runId: dispatchedRun.run_id }}
            to="/runs/$runId"
          >
            {dispatchedRun.run_id}
          </Link>{" "}
          — check the runs console for live progress.
        </Alert>
      ) : null}

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.4fr)_minmax(0,0.9fr)]">
        <SurfaceCard data-testid="review-detail-document">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>{document.kind}</SurfaceCardEyebrow>
              <SurfaceCardTitle>Reviewer comment &amp; patch</SurfaceCardTitle>
              <SurfaceCardDescription>
                Daemon-rendered review document. Updated {formatTimestamp(document.updated_at)}.
              </SurfaceCardDescription>
            </div>
          </SurfaceCardHeader>
          <SurfaceCardBody>
            {markdown.length === 0 ? (
              <EmptyState
                data-testid="review-detail-document-empty"
                title="No review document body available"
              />
            ) : (
              <div
                className="max-h-[min(72dvh,820px)] overflow-auto rounded-[var(--radius-lg)] border border-border-subtle bg-[color:var(--surface-inset)] px-5 py-4 shadow-[var(--shadow-xs)]"
                data-testid="review-detail-document-body"
              >
                <Markdown>{markdown}</Markdown>
              </div>
            )}
          </SurfaceCardBody>
          <SurfaceCardFooter>
            <span className="text-xs text-muted-foreground">issue #{issue.issue_number}</span>
            {isRefreshing ? (
              <span
                className="text-xs text-muted-foreground"
                data-testid="review-detail-refreshing"
              >
                refreshing…
              </span>
            ) : null}
          </SurfaceCardFooter>
        </SurfaceCard>

        <aside className="space-y-4" data-testid="review-detail-sidebar">
          <SurfaceCard data-testid="review-detail-meta">
            <SurfaceCardHeader>
              <div>
                <SurfaceCardEyebrow>Metadata</SurfaceCardEyebrow>
                <SurfaceCardTitle>Round context</SurfaceCardTitle>
                <SurfaceCardDescription>
                  Review round and provider that produced this issue.
                </SurfaceCardDescription>
              </div>
            </SurfaceCardHeader>
            <SurfaceCardBody>
              <dl className="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-2 text-xs">
                <dt className="font-eyebrow uppercase tracking-[0.14em] text-muted-foreground">
                  Workflow
                </dt>
                <dd className="truncate text-foreground">{workflow.slug}</dd>
                <dt className="font-eyebrow uppercase tracking-[0.14em] text-muted-foreground">
                  Round
                </dt>
                <dd className="text-foreground" data-testid="review-detail-round-number">
                  {round.round_number}
                </dd>
                <dt className="font-eyebrow uppercase tracking-[0.14em] text-muted-foreground">
                  PR
                </dt>
                <dd className="truncate text-foreground" data-testid="review-detail-pr">
                  {round.pr_ref ?? "—"}
                </dd>
                <dt className="font-eyebrow uppercase tracking-[0.14em] text-muted-foreground">
                  Provider
                </dt>
                <dd className="truncate text-foreground" data-testid="review-detail-provider">
                  {round.provider ?? "—"}
                </dd>
                <dt className="font-eyebrow uppercase tracking-[0.14em] text-muted-foreground">
                  Unresolved
                </dt>
                <dd className="text-foreground" data-testid="review-detail-unresolved">
                  {round.unresolved_count}
                </dd>
                <dt className="font-eyebrow uppercase tracking-[0.14em] text-muted-foreground">
                  Resolved
                </dt>
                <dd className="text-foreground" data-testid="review-detail-resolved">
                  {round.resolved_count}
                </dd>
              </dl>
            </SurfaceCardBody>
          </SurfaceCard>

          <RelatedRunsCard runs={payload.related_runs ?? []} />
        </aside>
      </div>
    </div>
  );
}

function RelatedRunsCard({ runs }: { runs: Run[] }): ReactElement {
  return (
    <SurfaceCard data-testid="review-detail-related-runs">
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Related runs</SurfaceCardEyebrow>
          <SurfaceCardTitle>Review fix runs</SurfaceCardTitle>
          <SurfaceCardDescription>
            Daemon runs associated with this review issue, most recent first.
          </SurfaceCardDescription>
        </div>
        <StatusBadge tone="info">{runs.length}</StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {runs.length === 0 ? (
          <EmptyState
            className="py-5"
            data-testid="review-detail-related-runs-empty"
            title="No related runs yet"
          />
        ) : (
          <ul className="space-y-2" data-testid="review-detail-related-runs-list">
            {runs.map(run => (
              <li
                className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover"
                data-testid={`review-detail-run-row-${run.run_id}`}
                key={run.run_id}
              >
                <div className="min-w-0 space-y-1">
                  <Link
                    className="truncate text-sm font-medium text-foreground hover:underline"
                    data-testid={`review-detail-run-link-${run.run_id}`}
                    params={{ runId: run.run_id }}
                    to="/runs/$runId"
                  >
                    {run.run_id}
                  </Link>
                  <p className="truncate text-xs text-muted-foreground">
                    {run.mode} · started {formatTimestamp(run.started_at)}
                    {run.ended_at ? ` · ended ${formatTimestamp(run.ended_at)}` : " · in flight"}
                  </p>
                </div>
                <StatusBadge
                  data-testid={`review-detail-run-status-${run.run_id}`}
                  tone={resolveRunStatusTone(run.status)}
                >
                  {run.status}
                </StatusBadge>
              </li>
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
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
