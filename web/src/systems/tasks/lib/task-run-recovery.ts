import type { TaskRunStatus } from "../types";

interface TaskRunRecoveryState {
  attempt: number;
  status?: TaskRunStatus | null;
}

export function taskRunCanRecover(
  run: TaskRunRecoveryState,
  maxAttempts: number | null | undefined
): boolean {
  return (
    run.status === "needs_attention" && typeof maxAttempts === "number" && run.attempt < maxAttempts
  );
}
