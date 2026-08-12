import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { deleteTaskExecutionProfile, setTaskExecutionProfile } from "../adapters/tasks-api";
import { taskExecutionProfileOptions } from "../lib/query-options";
import { tasksKeys } from "../lib/query-keys";
import { acknowledgeTaskMutationSettlement } from "../lib/task-mutation";
import type { TaskExecutionProfileSetRequest } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";

interface SetExecutionProfileParams {
  id: string;
  data: TaskExecutionProfileSetRequest;
}

interface DeleteExecutionProfileParams {
  id: string;
}

type QueryClient = ReturnType<typeof useQueryClient>;

function invalidateProfileRelatedQueries(queryClient: QueryClient, taskId: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: tasksKeys.profile(taskId) }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.detail(taskId) }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.agentContextsRoot() }),
  ]);
}

export function useTaskExecutionProfile(taskId: string, options: TaskQueryHookOptions = {}) {
  return useQuery(
    withTaskQueryHookOptions(taskExecutionProfileOptions(taskId, options.enabled ?? true), options)
  );
}

export function useSetTaskExecutionProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: SetExecutionProfileParams) => setTaskExecutionProfile(id, data),
    onSettled: (_result, error, { id }) => {
      acknowledgeTaskMutationSettlement(error);
      return invalidateProfileRelatedQueries(queryClient, id);
    },
  });
}

export function useDeleteTaskExecutionProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: DeleteExecutionProfileParams) => deleteTaskExecutionProfile(id),
    onSettled: (_result, error, { id }) => {
      acknowledgeTaskMutationSettlement(error);
      return invalidateProfileRelatedQueries(queryClient, id);
    },
  });
}
