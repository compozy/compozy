import { Time } from "@compozy/ui";

import { computeElapsed } from "../lib/task-formatters";
import type { TaskRun, TaskRunDetailView } from "../types";
import { TaskStateBand } from "./task-state-band";

type TaskRunRecord = TaskRunDetailView["run"];

/**
 * Outcome band for failed, completed, or attention-required attempts.
 * It renders nothing while the run is live because the head already owns status.
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.9
 */
export function TaskRunOutcome({
  run,
  nextAttempt,
}: {
  run: TaskRunDetailView;
  nextAttempt: TaskRun | null;
}) {
  const record = run.run;
  if (record.status === "failed") {
    return <TaskRunFailedOutcome nextAttempt={nextAttempt} record={record} />;
  }
  if (record.status === "completed") {
    return <TaskRunCompletedOutcome record={record} />;
  }
  if (record.status === "needs_attention") {
    return <TaskRunStuckOutcome record={record} />;
  }
  return null;
}

function TaskRunFailedOutcome({
  record,
  nextAttempt,
}: {
  record: TaskRunRecord;
  nextAttempt: TaskRun | null;
}) {
  const duration = computeElapsed(record);
  return (
    <TaskStateBand
      body={
        <>
          {record.error ?? "The run ended before reaching a checkpoint."}
          {nextAttempt ? ` Attempt ${nextAttempt.attempt} was queued from this run.` : null}
        </>
      }
      data-testid="tasks-run-outcome-failed"
      title={duration ? `Failed after ${duration}` : "Failed"}
      tone="danger"
    />
  );
}

function TaskRunCompletedOutcome({ record }: { record: TaskRunRecord }) {
  const duration = computeElapsed(record);
  const resultMessage =
    record.result == null ? "No result was recorded." : "The result is ready below.";
  return (
    <TaskStateBand
      body={
        record.ended_at ? (
          <>
            Finished <Time iso={record.ended_at} mode="relative" />
            {duration ? ` in ${duration}` : null}. {resultMessage}
          </>
        ) : (
          resultMessage
        )
      }
      data-testid="tasks-run-outcome-completed"
      title="Completed"
      tone="success"
    />
  );
}

function TaskRunStuckOutcome({ record }: { record: TaskRunRecord }) {
  return (
    <TaskStateBand
      body={
        record.error ?? "This attempt got stuck. Try again from the task to start a fresh attempt."
      }
      data-testid="tasks-run-outcome-stuck"
      title="This attempt needs attention"
      tone="danger"
    />
  );
}
