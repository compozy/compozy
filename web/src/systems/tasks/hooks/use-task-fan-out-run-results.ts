import { useQueries } from "@tanstack/react-query";

import { taskRunDetailOptions } from "../lib/query-options";
import type { FanOutTaskRunsResponse, TaskRun } from "../types";
import { useProfileReadScope } from "@/systems/profiles";

type AcceptedFanOutRun = FanOutTaskRunsResponse["runs"][number];

/**
 * Refresh only the runs accepted by the current fan-out request.
 *
 * The accepted response is the row authority; exact run-detail queries replace
 * its provisional values as materialization and execution advance. A response
 * for another task is ignored even if a malformed cache entry reused the id.
 */
export function useTaskFanOutRunResults(
  taskId: string,
  acceptedRuns: readonly AcceptedFanOutRun[]
): readonly TaskRun[] {
  const { params } = useProfileReadScope();
  const queries = useQueries({
    queries: acceptedRuns.map(run => taskRunDetailOptions(run.id, params, Boolean(taskId))),
  });

  return queries.flatMap((query, index) => {
    const accepted = acceptedRuns[index];
    const liveRun = query.data?.run;
    return accepted && liveRun?.id === accepted.id && liveRun.task_id === taskId ? [liveRun] : [];
  });
}
