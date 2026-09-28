import { AlertCircle, ChevronRight, History } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { cn, Empty, Pill, Section, SkeletonRows } from "@compozy/ui";

import {
  automationRunSkipReason,
  automationSkipReasonDetail,
  automationSkipReasonLabel,
  automationSkipReasonTone,
  automationRunStatusLabel,
  automationStatusTone,
  formatDateTime,
  formatRunDuration,
} from "../lib/automation-formatters";
import { automationRunDestination } from "../lib/automation-run-destination";
import type { AutomationRun } from "../types";

interface AutomationRunHistoryProps {
  emptyDescription?: string;
  emptyTitle?: string;
  error: Error | null;
  isLoading: boolean;
  loopWorkspaceId?: string;
  runs: AutomationRun[];
  title?: string;
}

interface AutomationRunRowProps {
  loopWorkspaceId?: string;
  run: AutomationRun;
}

const RUN_ROW_CLASS = "flex min-w-0 items-start gap-4 px-4 py-3";
const RUN_LINK_CLASS = cn(
  RUN_ROW_CLASS,
  "group/run-row text-left text-fg transition-colors duration-base ease-out hover:bg-hover focus-visible:bg-hover focus-visible:outline-none focus-visible:shadow-focus-inset"
);

function RunRowChevron() {
  return (
    <ChevronRight
      aria-hidden="true"
      className="ml-2 mt-1 size-3 shrink-0 text-subtle transition-colors duration-base ease-out group-hover/run-row:text-fg"
      strokeWidth={1.75}
    />
  );
}

function AutomationRunRow({ loopWorkspaceId, run }: AutomationRunRowProps) {
  const tone = automationStatusTone(run.status);
  const pulse = run.status === "running";
  const startedAt = formatDateTime(run.started_at);
  const duration = formatRunDuration(run);
  const statusLabel = automationRunStatusLabel(run.status);
  // A durable skip (self-overlap / grace exceeded) is a canceled run that never
  // dispatched: it keeps the canceled status but explains why, and drops the
  // started/duration slot.
  const skipReason = automationRunSkipReason(run);
  const testId = `automation-run-${run.id}`;
  const ariaLabel = skipReason
    ? `${statusLabel} run · attempt ${run.attempt} · ${automationSkipReasonDetail(skipReason)}`
    : `${statusLabel} run · attempt ${run.attempt} · started ${startedAt} · duration ${duration}`;
  const destination = automationRunDestination(run, loopWorkspaceId);

  const body = (
    <>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <Pill tone={tone}>
            <Pill.Dot pulse={pulse} tone={tone} />
            {statusLabel}
          </Pill>
          {skipReason ? (
            <Pill
              data-testid="automation-run-skip-reason"
              tone={automationSkipReasonTone(skipReason)}
            >
              {automationSkipReasonLabel(skipReason)}
            </Pill>
          ) : null}
          {run.attempt > 1 ? (
            <span className="text-form-hint text-subtle">{`Attempt ${run.attempt}`}</span>
          ) : null}
        </div>
        {skipReason ? (
          <p className="text-xs leading-relaxed text-muted">
            {automationSkipReasonDetail(skipReason)}
          </p>
        ) : null}
        {run.error ? <p className="text-xs leading-relaxed text-danger">{run.error}</p> : null}
        {run.delivery_error ? (
          <p className="text-xs leading-relaxed text-danger">{`Delivery: ${run.delivery_error}`}</p>
        ) : null}
      </div>
      <div className="flex shrink-0 flex-col items-end gap-1 text-right">
        {skipReason ? null : <span className="text-small-body text-muted">{startedAt}</span>}
        {run.scheduled_at ? (
          <span className="text-form-hint text-subtle">
            {`Scheduled ${formatDateTime(run.scheduled_at)}`}
          </span>
        ) : null}
        {skipReason ? null : <span className="text-form-hint text-subtle">{duration}</span>}
      </div>
    </>
  );

  if (destination?.kind === "loop-run") {
    return (
      <Link
        aria-label={`${ariaLabel} · Loop run ${destination.id}`}
        className={RUN_LINK_CLASS}
        data-testid={testId}
        params={{ runId: destination.id }}
        search={destination.workspaceId ? { workspace: destination.workspaceId } : {}}
        to="/loop-runs/$runId"
      >
        {body}
        <RunRowChevron />
      </Link>
    );
  }

  if (destination?.kind === "session") {
    return (
      <Link
        aria-label={ariaLabel}
        className={RUN_LINK_CLASS}
        data-testid={testId}
        params={{ id: destination.id }}
        to="/session/$id"
      >
        {body}
        <RunRowChevron />
      </Link>
    );
  }

  return (
    <div aria-label={ariaLabel} className={RUN_ROW_CLASS} data-testid={testId}>
      {body}
    </div>
  );
}

export function AutomationRunHistory({
  emptyDescription = "Runs will appear here after the first execution.",
  emptyTitle = "No runs recorded yet",
  error,
  isLoading,
  loopWorkspaceId,
  runs,
  title = "Runs",
}: AutomationRunHistoryProps) {
  return (
    <Section data-testid="automation-run-history" label={title} count={runs.length}>
      {isLoading ? (
        <SkeletonRows
          className="gap-4 rounded-lg bg-canvas-soft px-4 py-4"
          count={3}
          data-testid="automation-run-history-loading"
        />
      ) : error ? (
        <div className="flex justify-center px-2 py-6" data-testid="automation-run-history-error">
          <Empty
            description={error.message ?? "Something went wrong loading the runs."}
            icon={AlertCircle}
            title="Couldn't load runs"
            fill={false}
          />
        </div>
      ) : runs.length === 0 ? (
        <div className="flex justify-center px-2 py-6" data-testid="automation-run-history-empty">
          <Empty description={emptyDescription} icon={History} title={emptyTitle} fill={false} />
        </div>
      ) : (
        <ul
          className="overflow-hidden rounded-lg bg-canvas-soft"
          data-testid="automation-run-history-rows"
        >
          {runs.map(run => (
            <li className="border-b border-line last:border-b-0" key={run.id}>
              <AutomationRunRow loopWorkspaceId={loopWorkspaceId} run={run} />
            </li>
          ))}
        </ul>
      )}
    </Section>
  );
}
