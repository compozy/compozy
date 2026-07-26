import { Button, Time } from "@agh/ui";

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

/** Terminal-state renderer kept separate from live/blocking Now-strip orchestration. */
export function TaskNowTerminalState({
  detail,
  runs,
  onOpenRun,
  onViewResult,
}: TaskNowTerminalStateProps) {
  const record = detail.task;

  if (record.status === "failed") {
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
              className="min-h-6"
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
          failed?.error
            ? `${failed.error} Retry to queue a new attempt, or open the run to see what happened.`
            : "Retry to queue a new attempt, or open the run to see what happened."
        }
        data-testid="tasks-detail-now-failed"
        micro={failed ? `task.run_failed · ${failed.id}` : undefined}
        title={`Failed on ${attempts}`}
        tone="danger"
      />
    );
  }

  if (record.status === "completed") {
    const completed = latestTaskRun(runs, "completed");
    const duration = completed ? computeElapsed(completed) : undefined;
    const attemptCount = completed?.attempt ?? taskHighestAttemptOrdinal(runs, null);
    return (
      <TaskStateBand
        actions={
          completed ? (
            <Button
              className="min-h-6"
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
            Finished{" "}
            {record.closed_at ? <Time iso={record.closed_at} mode="relative" /> : "recently"}
            {duration ? ` in ${duration}` : null}.
          </>
        }
        data-testid="tasks-detail-now-completed"
        title={`Completed in ${attemptCount} ${attemptCount === 1 ? "attempt" : "attempts"}`}
        tone="success"
      />
    );
  }

  if (record.status === "canceled") {
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

  return null;
}
