import { Time } from "@agh/ui";

import { computeElapsed } from "../lib/task-formatters";
import type { TaskRun, TaskRunDetailView } from "../types";
import { TaskStateBand } from "./task-state-band";

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
  const duration = computeElapsed(record);

  if (record.status === "failed") {
    return (
      <TaskStateBand
        body={
          <>
            {record.error ?? "The run ended before reaching a checkpoint."}
            {nextAttempt ? ` Attempt ${nextAttempt.attempt} was queued from this run.` : null}
          </>
        }
        data-testid="tasks-run-outcome-failed"
        micro={`task.run_failed · ${record.id}`}
        title={duration ? `Failed after ${duration}` : "Failed"}
        tone="danger"
      />
    );
  }

  if (record.status === "completed") {
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

  if (record.status === "needs_attention") {
    return (
      <TaskStateBand
        body={
          record.error ??
          "This attempt requires operator attention. Recover requeues the work as a fresh attempt."
        }
        data-testid="tasks-run-outcome-stuck"
        micro={`task.run_needs_attention · ${record.id}`}
        title="This attempt needs attention"
        tone="danger"
      />
    );
  }

  return null;
}
