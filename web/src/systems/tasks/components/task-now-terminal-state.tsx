import { Button, Time } from "@compozy/ui";

import { taskHighestAttemptOrdinal } from "../lib/task-command-state";
import { computeElapsed } from "../lib/task-formatters";
import { latestTaskRun } from "../lib/task-run-presentation";
import type { TaskDetailView, TaskRun } from "../types";
import { TaskStateBand } from "./task-state-band";

interface TaskNowTerminalStateProps {
  detail: TaskDetailView;
  runs: readonly TaskRun[];
  onOpenRun: (runId: string) => void;
  onViewResult: () => void;
}

type TaskRecord = TaskDetailView["task"];

/** Terminal-state renderer kept separate from live/blocking Now-strip orchestration. */
export function TaskNowTerminalState({
  detail,
  runs,
  onOpenRun,
  onViewResult,
}: TaskNowTerminalStateProps) {
  const record = detail.task;
  if (record.status === "failed") {
    return <TaskNowFailedState onOpenRun={onOpenRun} record={record} runs={runs} />;
  }
  if (record.status === "completed") {
    return <TaskNowCompletedState onViewResult={onViewResult} record={record} runs={runs} />;
  }
  if (record.status === "canceled") {
    return <TaskNowCanceledState record={record} />;
  }
  return null;
}

function TaskNowFailedState({
  record,
  runs,
  onOpenRun,
}: {
  record: TaskRecord;
  runs: readonly TaskRun[];
  onOpenRun: (runId: string) => void;
}) {
  const failed = latestTaskRun(runs, "failed");
  const highestAttempt = taskHighestAttemptOrdinal(runs, null);
  const attempts = record.max_attempts
    ? `attempt ${highestAttempt} of ${record.max_attempts}`
    : `attempt ${highestAttempt}`;
  return (
    <TaskStateBand
      actions={
        failed ? (
          <Button
            data-testid="tasks-detail-now-open-failed-run"
            onClick={() => onOpenRun(failed.id)}
            size="sm"
            type="button"
            variant="neutral"
          >
            Open run
          </Button>
        ) : undefined
      }
      body={
        // The runtime error is quoted verbatim on its own line: it carries no
        // punctuation contract, so appending the guidance sentence to it would
        // run the two together.
        failed?.error ? (
          <>
            <span className="block" data-testid="tasks-detail-now-failed-error">
              {failed.error}
            </span>
            {FAILED_GUIDANCE}
          </>
        ) : (
          FAILED_GUIDANCE
        )
      }
      data-testid="tasks-detail-now-failed"
      title={`Failed on ${attempts}`}
      tone="danger"
    />
  );
}

const FAILED_GUIDANCE = "Retry to queue a new attempt, or open the run to see what happened.";

function TaskNowCompletedState({
  record,
  runs,
  onViewResult,
}: {
  record: TaskRecord;
  runs: readonly TaskRun[];
  onViewResult: () => void;
}) {
  const completed = latestTaskRun(runs, "completed");
  const duration = completed ? computeElapsed(completed) : undefined;
  const attemptCount = completed?.attempt ?? taskHighestAttemptOrdinal(runs, null);
  return (
    <TaskStateBand
      actions={
        completed ? (
          <Button
            data-testid="tasks-detail-now-view-result"
            onClick={onViewResult}
            size="sm"
            type="button"
            variant="neutral"
          >
            View result
          </Button>
        ) : undefined
      }
      body={
        <>
          Finished {record.closed_at ? <Time iso={record.closed_at} mode="relative" /> : "recently"}
          {duration ? ` in ${duration}` : null}.
        </>
      }
      data-testid="tasks-detail-now-completed"
      title={`Completed in ${attemptCount} ${attemptCount === 1 ? "attempt" : "attempts"}`}
      tone="success"
    />
  );
}

function TaskNowCanceledState({ record }: { record: TaskRecord }) {
  return (
    <TaskStateBand
      body={
        record.closed_at ? (
          <>
            Canceled <Time iso={record.closed_at} mode="relative" />. No further runs will start.
          </>
        ) : (
          "No further runs will start."
        )
      }
      data-testid="tasks-detail-now-canceled"
      title="Canceled"
      tone="neutral"
    />
  );
}
