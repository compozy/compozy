import { Link } from "@tanstack/react-router";
import { ArrowUpRight, Search } from "lucide-react";
import type * as React from "react";

import { Button, cn, OwnerAvatar, Pill, PropertyRow, Time } from "@compozy/ui";

import { describeCost } from "@/lib/cost-provenance";
import { ownerAvatarKindFor, taskRunStatusLabel, taskRunStatusTone } from "../lib/task-formatters";
import { taskRunLineage } from "../lib/task-run-presentation";
import type { TaskRun, TaskRunDetailView } from "../types";
import { TaskRailSection } from "./task-rail-section";

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
 * Run-detail rail: session link, who worked on it, cost, timing, and attempt
 * lineage. Ids, counters, and claim internals stay behind Inspect.
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
      <TaskRailSection label="Session">
        {sessionId ? (
          <PropertyRow
            editor={
              <Link
                className="inline-flex min-h-6 min-w-0 items-center gap-1 rounded-sm px-1.5 py-0.5 text-small-body font-medium text-fg hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
                data-testid="tasks-run-rail-session"
                params={{ id: sessionId }}
                to="/session/$id"
              >
                Open session
                <ArrowUpRight aria-hidden="true" className="size-3" />
              </Link>
            }
            label="Session"
          />
        ) : (
          <PropertyRow label="Session">
            <span className="text-muted">Not attached</span>
          </PropertyRow>
        )}
        {claimant ? (
          <PropertyRow label="Worked on by">
            <OwnerAvatar
              name={claimant}
              ownerId={claimant}
              ownerKind={ownerAvatarKindFor(record.claimed_by?.kind)}
              size="sm"
            />
            <span className="truncate">{claimant}</span>
          </PropertyRow>
        ) : null}
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
      </TaskRailSection>

      <TaskRailSection label="Timing">
        <PropertyRow label="Queued">
          <Time iso={record.queued_at} mode="relative" />
        </PropertyRow>
        {record.claimed_at ? (
          <PropertyRow label="Picked up">
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
      </TaskRailSection>

      <TaskRailSection aria-busy={taskRunsLoading || undefined} label="Other attempts">
        {taskRunsLoading ? (
          <p className="py-1 text-form-label text-muted" role="status">
            Loading attempts…
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
      </TaskRailSection>

      <footer className="flex items-center gap-2 border-t border-line-soft px-3 py-2.5">
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
      </footer>
    </div>
  );
}
