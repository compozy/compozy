import { Link } from "@tanstack/react-router";
import { Search } from "lucide-react";
import type * as React from "react";

import { Button, cn, MonoId, OwnerAvatar, Pill, PropertyRow, Time } from "@agh/ui";

import { describeCost } from "@/lib/cost-provenance";
import { ownerAvatarKindFor, taskRunStatusLabel, taskRunStatusTone } from "../lib/task-formatters";
import { formatTaskRunMetric, taskRunLineage } from "../lib/task-run-presentation";
import type { TaskRun, TaskRunDetailView } from "../types";

export interface TaskRunRailProps extends React.ComponentProps<"div"> {
  run: TaskRunDetailView;
  taskId: string;
  /** Sibling runs of the parent task, used for lineage rows. */
  taskRuns: readonly TaskRun[];
  taskRunsLoading?: boolean;
  taskRunsErrorMessage?: string | null;
  onInspect: () => void;
  duration?: string;
}

const METRIC_PLACEHOLDER = "—";

function LineageRow({ label, taskId, target }: { label: string; taskId: string; target: TaskRun }) {
  return (
    <PropertyRow
      editor={
        <Link
          className="inline-flex min-h-6 min-w-6 items-center gap-1.5 rounded-sm px-1.5 py-0.5 text-small-body font-medium text-fg hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
          data-testid={`tasks-run-lineage-${target.id}`}
          params={{ id: taskId, runId: target.id }}
          to="/tasks/$id/runs/$runId"
        >
          <Pill.Dot tone={taskRunStatusTone(target.status)} />
          <span className="truncate">
            Attempt {target.attempt} · {taskRunStatusLabel(target.status)}
          </span>
        </Link>
      }
      label={label}
    />
  );
}

/**
 * Run-detail rail: session binding, best-effort operational metrics
 * ("—" when the runtime has no number, never an invented zero), timing, and
 * attempt lineage. Tier-c internals stay behind Inspect.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.9
 */
export function TaskRunRail({
  run,
  taskId,
  taskRuns,
  taskRunsLoading = false,
  taskRunsErrorMessage = null,
  onInspect,
  duration,
  className,
  ...props
}: TaskRunRailProps) {
  const record = run.run;
  const session = run.session ?? null;
  const summary = run.summary ?? null;
  const sessionId = record.session_id ?? session?.session_id ?? null;
  const claimant = record.claimed_by?.ref ?? null;
  const cost = describeCost({
    status: summary?.cost_status,
    source: summary?.cost_source,
    amount: summary?.total_cost,
    currency: summary?.cost_currency,
  });
  const { previous, next } = taskRunLineage(record, taskRuns);
  const hasLineage = Boolean(previous || next);

  return (
    <div
      {...props}
      className={cn("overflow-hidden rounded-lg border border-line bg-canvas-soft", className)}
      data-testid="tasks-run-rail"
    >
      <section className="border-t border-line-soft px-4 py-3.5 first:border-t-0">
        <h3 className="eyebrow mb-2 text-subtle">Session</h3>
        {sessionId ? (
          <PropertyRow label="Session" mono>
            <MonoId value={sessionId} />
          </PropertyRow>
        ) : (
          <PropertyRow label="Session">
            <span className="text-muted">Not attached</span>
          </PropertyRow>
        )}
        {claimant ? (
          <PropertyRow label="Claimed by">
            <OwnerAvatar
              name={claimant}
              ownerId={claimant}
              ownerKind={ownerAvatarKindFor(record.claimed_by?.kind)}
              size="sm"
            />
            <span className="truncate">{claimant}</span>
          </PropertyRow>
        ) : null}
        <PropertyRow label="Tool calls" mono>
          {formatTaskRunMetric(summary?.tool_call_count)}
        </PropertyRow>
        <PropertyRow label="Turns" mono>
          {formatTaskRunMetric(summary?.turn_count)}
        </PropertyRow>
        <PropertyRow label="Tokens" mono>
          {formatTaskRunMetric(summary?.total_tokens)}
        </PropertyRow>
        {cost.hasCost ? (
          <PropertyRow
            data-cost-status={cost.status}
            data-testid="task-run-detail-cost"
            label={cost.isEstimated ? "Est. cost" : "Cost"}
            mono
          >
            <span>{cost.value}</span>
            {cost.note ? <span className="font-sans text-form-label">{cost.note}</span> : null}
          </PropertyRow>
        ) : null}
      </section>

      <section className="border-t border-line-soft px-4 py-3.5">
        <h3 className="eyebrow mb-2 text-subtle">Timing</h3>
        <PropertyRow label="Queued">
          <Time iso={record.queued_at} mode="relative" />
        </PropertyRow>
        {record.claimed_at ? (
          <PropertyRow label="Claimed">
            <Time iso={record.claimed_at} mode="relative" />
          </PropertyRow>
        ) : null}
        {record.started_at ? (
          <PropertyRow label="Started">
            <Time iso={record.started_at} mode="relative" />
          </PropertyRow>
        ) : null}
        {record.ended_at ? (
          <PropertyRow label="Ended">
            <Time iso={record.ended_at} mode="relative" />
          </PropertyRow>
        ) : null}
        <PropertyRow label={record.ended_at ? "Duration" : "Elapsed"} mono>
          {duration ?? METRIC_PLACEHOLDER}
        </PropertyRow>
      </section>

      <section
        aria-busy={taskRunsLoading || undefined}
        className="border-t border-line-soft px-4 py-3.5"
      >
        <h3 className="eyebrow mb-2 text-subtle">Lineage</h3>
        {taskRunsLoading ? (
          <p className="py-1 text-form-label text-muted" role="status">
            Loading lineage…
          </p>
        ) : null}
        {taskRunsErrorMessage ? (
          <p className="py-1 text-form-label text-danger" role="alert">
            {taskRunsErrorMessage}
          </p>
        ) : null}
        {!taskRunsLoading ? (
          <>
            {previous ? <LineageRow label="Previous" target={previous} taskId={taskId} /> : null}
            {next ? <LineageRow label="Next" target={next} taskId={taskId} /> : null}
            {!taskRunsErrorMessage && !hasLineage ? (
              <PropertyRow label="Attempts">
                <span className="text-muted">No linked attempts</span>
              </PropertyRow>
            ) : null}
          </>
        ) : null}
        <PropertyRow label="Run id" mono>
          <MonoId value={record.id} />
        </PropertyRow>
      </section>

      <footer className="flex items-center justify-between gap-2 border-t border-line-soft px-3 py-2.5">
        <Button
          className="min-h-6"
          data-testid="tasks-run-inspect"
          onClick={onInspect}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Search aria-hidden="true" className="size-3" />
          Inspect
        </Button>
        <span className="truncate font-mono text-micro text-faint">
          agh task run show {record.id}
        </span>
      </footer>
    </div>
  );
}
