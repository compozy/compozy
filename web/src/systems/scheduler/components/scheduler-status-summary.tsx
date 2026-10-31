import { Pill, SkeletonRegion, SkeletonText, Time } from "@compozy/ui";

import { schedulerStatusCounts } from "../lib/scheduler-status-view";
import type { SchedulerStatus } from "../types";

interface SchedulerStatusSummaryProps {
  status: SchedulerStatus | null;
  /** The first read of the status is pending. A refresh with a status in hand shows nothing. */
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
      </div>
      {isInitialStatusLoading ? (
        <SchedulerStatusMetaSkeleton />
      ) : (
        <SchedulerStatusMeta status={status} />
      )}
      {status?.paused_reason ? (
        <p
          className="mt-2 max-w-3xl text-form-label text-muted-foreground"
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
      className="inline-flex items-center gap-1.5 text-form-label text-muted-foreground"
      data-testid="scheduler-controls-state"
    >
      {isInitialLoading ? (
        <SkeletonText className="w-14" />
      ) : (
        <>
          <Pill.Dot tone={paused ? "neutral" : "success"} />
          {paused ? "Paused" : "Running"}
        </>
      )}
    </span>
  );
}

/** The counts line in skeleton form: one line of the line box the real counts take. */
function SchedulerStatusMetaSkeleton() {
  return (
    <SkeletonRegion
      className="mt-2 text-form-label"
      data-testid="scheduler-controls-meta-loading"
      label="Loading scheduler status"
    >
      <SkeletonText className="w-120" />
    </SkeletonRegion>
  );
}

function SchedulerStatusMeta({ status }: { status: SchedulerStatus | null }) {
  const counts = schedulerStatusCounts(status);

  return (
    <div
      className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-form-label text-muted-foreground"
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
        className={counts.needsAttention > 0 ? "text-brand" : undefined}
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
