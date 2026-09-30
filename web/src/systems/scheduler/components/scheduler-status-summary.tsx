import { Pill, Skeleton, Time } from "@compozy/ui";

import { schedulerStatusCounts } from "../lib/scheduler-status-view";
import type { SchedulerStatus } from "../types";

interface SchedulerStatusSummaryProps {
  status: SchedulerStatus | null;
  isLoading: boolean;
}

/** Title, running/paused state, queue counts, and pause reason of the task queue. */
export function SchedulerStatusSummary({ status, isLoading }: SchedulerStatusSummaryProps) {
  const isInitialStatusLoading = isLoading && !status;

  return (
    <div className="min-w-0">
      <div className="flex min-w-0 flex-wrap items-center gap-2.5">
        <h2 className="text-item-title font-medium text-fg-strong">Task queue</h2>
        <SchedulerStateLabel isInitialLoading={isInitialStatusLoading} paused={status?.paused} />
        {isLoading && status ? (
          <Pill data-testid="scheduler-controls-loading" tone="neutral">
            Loading
          </Pill>
        ) : null}
      </div>
      {isInitialStatusLoading ? (
        <SchedulerStatusMetaSkeleton />
      ) : (
        <SchedulerStatusMeta status={status} />
      )}
      {status?.paused_reason ? (
        <p
          className="mt-2 max-w-3xl text-form-label text-muted"
          data-testid="scheduler-controls-reason"
        >
          {status.paused_reason}
        </p>
      ) : null}
    </div>
  );
}

function SchedulerStateLabel({
  isInitialLoading,
  paused,
}: {
  isInitialLoading: boolean;
  paused: boolean | undefined;
}) {
  return (
    <span
      className="inline-flex items-center gap-1.5 text-form-label text-muted"
      data-testid="scheduler-controls-state"
    >
      {isInitialLoading ? null : <Pill.Dot tone={paused ? "neutral" : "success"} />}
      {isInitialLoading ? "Loading" : paused ? "Paused" : "Running"}
    </span>
  );
}

function SchedulerStatusMetaSkeleton() {
  return (
    <div
      aria-label="Loading scheduler status"
      className="mt-2 flex items-center gap-3"
      data-testid="scheduler-controls-meta-loading"
      role="status"
    >
      <Skeleton className="h-3 w-24" />
      <Skeleton className="h-3 w-20" />
      <Skeleton className="h-3 w-28" />
    </div>
  );
}

function SchedulerStatusMeta({ status }: { status: SchedulerStatus | null }) {
  const counts = schedulerStatusCounts(status);

  return (
    <div
      className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-form-label text-muted"
      data-testid="scheduler-controls-meta"
    >
      <span>{counts.running} running</span>
      <MetaDot />
      <span>{counts.waiting} waiting</span>
      <MetaDot />
      <span>{counts.paused} paused</span>
      <MetaDot />
      <span
        className={counts.starved > 0 ? "text-warning" : undefined}
        data-testid="scheduler-controls-starved-count"
      >
        {counts.starved} waiting too long
      </span>
      <MetaDot />
      <span
        className={counts.needsAttention > 0 ? "text-accent" : undefined}
        data-testid="scheduler-controls-needs-attention-count"
      >
        {counts.needsAttention} need attention
      </span>
      {status?.paused_at ? (
        <>
          <MetaDot />
          <span>
            Paused <Time iso={status.paused_at} mode="relative" />
          </span>
        </>
      ) : null}
    </div>
  );
}

function MetaDot() {
  return (
    <span aria-hidden="true" className="text-faint">
      ·
    </span>
  );
}
