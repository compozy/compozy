import { taskRunCanRecover } from "./task-run-recovery";
import { taskCanRecover, taskHasApprovalPending, taskIsDraft } from "./task-formatters";
import type { TaskDetailView, TaskRun, TaskRunStatus } from "../types";

/** The single accent action for a task, mapped to one runtime verb or navigation. */
export type TaskPrimaryCommand =
  | { kind: "publish" }
  | { kind: "approve" }
  | { kind: "start" }
  | { kind: "open_run"; runId: string }
  | { kind: "resume" }
  | { kind: "recover" }
  | { kind: "retry"; runId: string };

export interface TaskCommandState {
  primary: TaskPrimaryCommand | null;
  secondary: {
    edit: boolean;
    pause: boolean;
    reject: boolean;
  };
  /** Fan-out enqueues runs and therefore shares the one-open-run gate. */
  canFanOut: boolean;
  overflow: {
    edit: boolean;
    pause: boolean;
    resume: boolean;
    cancel: boolean;
    /** Enqueue an additional run from the overflow (failed-with-attempts path). */
    startNewRun: boolean;
    delete: boolean;
  };
}

function isOpenRunStatus(status: TaskRunStatus | undefined | null): boolean {
  return (
    status === "queued" || status === "claimed" || status === "starting" || status === "running"
  );
}

function latestFailedRun(runs: readonly TaskRun[]): TaskRun | null {
  let best: TaskRun | null = null;
  for (const run of runs) {
    if (run.status !== "failed") continue;
    if (!best || run.attempt > best.attempt) best = run;
  }
  return best;
}

export function taskHighestAttemptOrdinal(
  runs: readonly TaskRun[],
  activeRun: { attempt?: number } | null
): number {
  let highestAttempt = typeof activeRun?.attempt === "number" ? activeRun.attempt : 0;
  for (const run of runs) {
    if (typeof run.attempt === "number" && run.attempt > highestAttempt) {
      highestAttempt = run.attempt;
    }
  }
  return highestAttempt;
}

/**
 * Derives the primary-action state machine from runtime truth. Precedence:
 * recover > publish > approve > resume > open run > retry > start. States with
 * no truthful next verb (blocked, completed, canceled, failed-exhausted) render
 * no accent target — resolving actions live on the Now-strip cards instead.
 */
export function resolveTaskCommandState(
  detail: TaskDetailView,
  runs: readonly TaskRun[]
): TaskCommandState {
  const record = detail.task;
  const activeRun = detail.summary?.active_run ?? null;
  const isDraft = taskIsDraft(record);
  const isDirectlyPaused = Boolean(record.paused);
  const isEffectivelyPaused = Boolean(detail.summary?.effective_paused ?? record.paused);
  const approvalPending = taskHasApprovalPending(record);
  const activeRunNeedsAttention = activeRun?.status === "needs_attention";
  const canRecover = activeRunNeedsAttention
    ? taskRunCanRecover(activeRun, record.max_attempts)
    : taskCanRecover(record);
  const hasOpenRun = isOpenRunStatus(activeRun?.status) || activeRunNeedsAttention;
  const maxAttempts = record.max_attempts ?? null;
  const highestAttempt = taskHighestAttemptOrdinal(runs, activeRun);
  const attemptsRemain = maxAttempts === null || highestAttempt < maxAttempts;
  const failedRun = record.status === "failed" ? latestFailedRun(runs) : null;
  const isTerminal =
    record.status === "completed" ||
    record.status === "canceled" ||
    (record.status === "failed" && (!attemptsRemain || !failedRun));
  const canEdit = isDraft || record.status === "ready" || isDirectlyPaused || isTerminal;
  const canPause =
    !isDraft &&
    !isDirectlyPaused &&
    !isTerminal &&
    (record.status === "ready" || record.status === "in_progress" || record.status === "blocked");
  const canFanOut =
    !isDraft &&
    !hasOpenRun &&
    !isEffectivelyPaused &&
    !approvalPending &&
    (record.status === "pending" || record.status === "ready");

  let primary: TaskPrimaryCommand | null = null;
  if (record.status === "needs_attention" || activeRunNeedsAttention) {
    primary = canRecover ? { kind: "recover" } : null;
  } else if (isDraft) {
    primary = { kind: "publish" };
  } else if (approvalPending) {
    primary = { kind: "approve" };
  } else if (isDirectlyPaused) {
    primary = { kind: "resume" };
  } else if (activeRun && isOpenRunStatus(activeRun.status)) {
    primary = { kind: "open_run", runId: activeRun.id };
  } else if (record.status === "failed" && attemptsRemain && failedRun) {
    primary = { kind: "retry", runId: failedRun.id };
  } else if (
    !hasOpenRun &&
    !isEffectivelyPaused &&
    record.status !== "blocked" &&
    (record.status === "pending" || record.status === "ready")
  ) {
    primary = { kind: "start" };
  }

  const canCancel =
    record.status === "ready" || record.status === "in_progress" || record.status === "blocked";

  return {
    primary,
    secondary: {
      edit: canEdit,
      pause: canPause,
      reject: approvalPending,
    },
    canFanOut,
    overflow: {
      edit: !canEdit,
      pause: !canPause && !isDirectlyPaused && !isTerminal && !isDraft,
      resume: isDirectlyPaused,
      cancel: canCancel,
      startNewRun: record.status === "failed" && attemptsRemain && !hasOpenRun,
      delete: true,
    },
  };
}
