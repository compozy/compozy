import { useMutation, useQueryClient } from "@tanstack/react-query";

import { recoverTaskRun } from "../adapters/tasks-api";
import { tasksKeys } from "../lib/query-keys";
import { acknowledgeTaskMutationSettlement } from "../lib/task-mutation";
import type { RecoverTaskRunRequest } from "../types";
import { invalidateAggregateQueries, invalidateTaskQueries } from "./task-query-invalidation";

interface RecoverTaskRunParams {
  runId: string;
  taskId: string;
  data?: RecoverTaskRunRequest;
}

export function useRecoverTaskRun() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ runId, data }: RecoverTaskRunParams) => recoverTaskRun(runId, data ?? {}),
    onSettled: (_result, error, { runId, taskId }) => {
      acknowledgeTaskMutationSettlement(error);
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(runId) }),
        invalidateTaskQueries(queryClient, taskId),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}
