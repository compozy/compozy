import { Link } from "@tanstack/react-router";
import { AlertCircle, ChevronRight, CornerUpLeft, Play } from "lucide-react";

import {
  Button,
  cn,
  Empty,
  LinkedRecordTableBody,
  LinkedRecordTableCell,
  LinkedRecordTableOpenCell,
  LinkedRecordTableRoot,
  LinkedRecordTableRow,
  LinkedRecordTableTitle,
  OwnerAvatar,
  Pill,
  Spinner,
  Time,
} from "@agh/ui";

import { ownerAvatarKindFor, taskRunStatusLabel, taskRunStatusTone } from "../lib/task-formatters";
import { taskRunReviewPresentation } from "../lib/task-run-presentation";
import type { TaskRun, TaskRunReview } from "../types";
import { TaskRowsLoadingSkeleton } from "./task-loading-skeletons";

export interface TaskRunsPanelProps {
  taskId: string;
  runs: readonly TaskRun[];
  reviewsByRun?: ReadonlyMap<string, readonly TaskRunReview[]>;
  isLoading?: boolean;
  errorMessage?: string | null;
  /** Present only when the runtime allows starting a run right now. */
  onStartRun?: () => void;
  isStartPending?: boolean;
  workerName?: string | null;
  runDurations?: ReadonlyMap<string, string | undefined>;
  emptyDescription?: string;
}

const RUN_COLUMNS = ["Attempt", "Status", "Claimed by", "Started", "Duration", "Result"] as const;

function RunRow({
  taskId,
  run,
  lineageAttempt,
  reviews,
  duration,
}: {
  taskId: string;
  run: TaskRun;
  lineageAttempt: number | null;
  reviews: readonly TaskRunReview[];
  duration?: string;
}) {
  const isActive = run.status === "running" || run.status === "starting";
  const claimant = run.claimed_by?.ref;
  const resultText =
    run.status === "failed"
      ? (run.error ?? "Failed")
      : run.status === "completed"
        ? run.result != null
          ? "Result ready"
          : "Completed"
        : null;

  return (
    <>
      <LinkedRecordTableRow data-testid={`tasks-runs-row-${run.id}`}>
        <LinkedRecordTableCell className="w-8 pl-4">
          <Pill.Dot pulse={isActive} tone={taskRunStatusTone(run.status)} />
        </LinkedRecordTableCell>
        <LinkedRecordTableCell>
          <LinkedRecordTableTitle>
            <Link
              className="inline-flex min-h-6 items-center font-medium text-fg-strong hover:underline focus-visible:outline-none focus-visible:shadow-focus-ring"
              params={{ id: taskId, runId: run.id }}
              to="/tasks/$id/runs/$runId"
            >
              Attempt {run.attempt}
            </Link>
            {lineageAttempt !== null ? (
              <span className="flex items-center gap-1 text-eyebrow text-subtle">
                <CornerUpLeft aria-hidden="true" className="size-[11px] text-faint" />
                retried from attempt {lineageAttempt}
              </span>
            ) : null}
          </LinkedRecordTableTitle>
        </LinkedRecordTableCell>
        <LinkedRecordTableCell>
          <Pill tone={taskRunStatusTone(run.status)}>
            <Pill.Dot pulse={isActive} tone={taskRunStatusTone(run.status)} />
            {taskRunStatusLabel(run.status)}
          </Pill>
        </LinkedRecordTableCell>
        <LinkedRecordTableCell>
          <span className="inline-flex min-w-0 items-center gap-1.5 text-small-body text-muted">
            {claimant ? (
              <>
                <OwnerAvatar
                  name={claimant}
                  ownerId={claimant}
                  ownerKind={ownerAvatarKindFor(run.claimed_by?.kind)}
                  size="sm"
                />
                <span className="truncate">{claimant}</span>
              </>
            ) : (
              <span className="text-subtle">—</span>
            )}
          </span>
        </LinkedRecordTableCell>
        <LinkedRecordTableCell className="hidden text-form-label tabular-nums text-muted md:table-cell">
          {run.started_at ? <Time iso={run.started_at} mode="relative" /> : "—"}
        </LinkedRecordTableCell>
        <LinkedRecordTableCell className="hidden font-mono text-eyebrow tabular-nums text-muted md:table-cell">
          {duration ?? "—"}
        </LinkedRecordTableCell>
        <LinkedRecordTableCell
          className={cn(
            "hidden max-w-48 truncate text-form-label md:table-cell",
            run.status === "failed" ? "text-danger" : "text-muted"
          )}
        >
          {resultText ?? "—"}
        </LinkedRecordTableCell>
        <LinkedRecordTableOpenCell>
          <Link
            aria-label={`Open attempt ${run.attempt}`}
            className="inline-flex size-6 items-center justify-center rounded-sm text-faint hover:bg-row-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
            params={{ id: taskId, runId: run.id }}
            to="/tasks/$id/runs/$runId"
          >
            <ChevronRight aria-hidden="true" className="size-3.5" />
          </Link>
        </LinkedRecordTableOpenCell>
      </LinkedRecordTableRow>
      {reviews.length > 0 ? (
        <LinkedRecordTableRow>
          <LinkedRecordTableCell className="py-2 pl-12" colSpan={8}>
            <div className="flex flex-col gap-1">
              {reviews.map(review => {
                const presentation = taskRunReviewPresentation(review);
                return (
                  <p
                    className="flex items-center gap-2 truncate text-form-label text-subtle"
                    data-testid={`tasks-runs-review-${review.review_id}`}
                    key={review.review_id}
                  >
                    <Pill.Dot tone={presentation.tone} />
                    {presentation.line}
                  </p>
                );
              })}
            </div>
          </LinkedRecordTableCell>
        </LinkedRecordTableRow>
      ) : null}
    </>
  );
}

/**
 * Runs tab: attempts newest-first, whole row links to the run page,
 * reviews render as a quiet secondary line under their run — never a separate
 * table. Empty state teaches the next useful action.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.5
 */
export function TaskRunsPanel({
  taskId,
  runs,
  reviewsByRun,
  isLoading = false,
  errorMessage = null,
  onStartRun,
  isStartPending = false,
  workerName,
  runDurations,
  emptyDescription,
}: TaskRunsPanelProps) {
  if (isLoading && runs.length === 0) {
    return <TaskRowsLoadingSkeleton label="Loading runs" testId="tasks-runs-loading" />;
  }

  if (errorMessage && runs.length === 0) {
    return (
      <Empty
        icon={AlertCircle}
        title="Couldn't load runs"
        description={errorMessage}
        data-testid="tasks-runs-error"
      />
    );
  }

  if (runs.length === 0) {
    return (
      <Empty
        icon={Play}
        title="Not started yet"
        description={
          emptyDescription ??
          (workerName
            ? `Start a run to have ${workerName} work on this task.`
            : "Start a run to have a worker pick this task up.")
        }
        action={
          onStartRun ? (
            <Button
              aria-busy={isStartPending || undefined}
              data-testid="tasks-runs-start"
              disabled={isStartPending}
              onClick={onStartRun}
              size="sm"
              type="button"
              variant="neutral"
            >
              {isStartPending ? <Spinner aria-hidden="true" className="size-3" /> : null}
              {isStartPending ? "Starting…" : "Start run"}
            </Button>
          ) : undefined
        }
        data-testid="tasks-runs-empty"
      />
    );
  }

  const byAttempt = [...runs].sort((a, b) => b.attempt - a.attempt);
  const attemptById = new Map(runs.map(run => [run.id, run.attempt]));

  return (
    <div className="flex flex-col gap-2">
      {errorMessage ? (
        <p
          className="text-small-body text-danger"
          data-testid="tasks-runs-background-error"
          role="alert"
        >
          {errorMessage}
        </p>
      ) : null}
      <LinkedRecordTableRoot
        aria-busy={isLoading || undefined}
        className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
        columns={RUN_COLUMNS}
        data-testid="tasks-runs-panel"
      >
        <LinkedRecordTableBody>
          {byAttempt.map(run => (
            <RunRow
              duration={runDurations?.get(run.id)}
              key={run.id}
              lineageAttempt={
                run.previous_run_id ? (attemptById.get(run.previous_run_id) ?? null) : null
              }
              reviews={reviewsByRun?.get(run.id) ?? []}
              run={run}
              taskId={taskId}
            />
          ))}
        </LinkedRecordTableBody>
      </LinkedRecordTableRoot>
    </div>
  );
}
