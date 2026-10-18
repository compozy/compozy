import type * as React from "react";

import {
  latestTaskRun,
  type TaskOverviewPanel,
  type TaskRun,
  useTaskRunResult,
} from "@/systems/tasks";

export type TaskOverviewCompletedResult = React.ComponentProps<
  typeof TaskOverviewPanel
>["completedResult"];

/** Latest completed attempt and its external result, surfaced on the Overview tab. */
export function useCompletedRunResult(
  runs: readonly TaskRun[],
  workspaceId: string | undefined
): TaskOverviewCompletedResult {
  const completedRun = latestTaskRun(runs, "completed");
  const completedRunResult = useTaskRunResult({
    resultBytes: completedRun?.result_bytes ?? 0,
    resultRef: completedRun?.result_ref ?? "",
    runId: completedRun?.id ?? "",
    workspaceId: workspaceId ?? "",
  });
  if (!completedRun) return undefined;
  return {
    external: completedRun.result_ref ? completedRunResult : undefined,
    run: completedRun,
  };
}
