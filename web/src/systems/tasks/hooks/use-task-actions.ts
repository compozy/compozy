import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  approveTask,
  cancelTask,
  clearTaskBlock,
  createChildTask,
  createTask,
  deleteTask,
  enqueueTaskRun,
  fanOutTaskRuns,
  pauseTask,
  publishTask,
  recoverTask,
  rejectTask,
  resumeTask,
  updateTask,
} from "../adapters/tasks-api";
import { invalidateAggregateQueries, invalidateTaskQueries } from "./task-query-invalidation";
import { tasksKeys } from "../lib/query-keys";
import type {
  CancelTaskRequest,
  CreateChildTaskRequest,
  CreateTaskRequest,
  EnqueueTaskRunRequest,
  FanOutTaskRunsRequest,
  PauseTaskRequest,
  RecoverTaskRequest,
  ResumeTaskRequest,
  UpdateTaskRequest,
} from "../types";

interface TaskIdParams {
  id: string;
}

interface UpdateTaskParams extends TaskIdParams {
  data: UpdateTaskRequest;
}

interface CancelTaskParams extends TaskIdParams {
  data?: CancelTaskRequest;
}

interface PauseTaskParams extends TaskIdParams {
  data: PauseTaskRequest;
}

interface ResumeTaskParams extends TaskIdParams {
  data?: ResumeTaskRequest;
}

interface RecoverTaskParams extends TaskIdParams {
  data?: RecoverTaskRequest;
}

interface ClearTaskBlockParams extends TaskIdParams {
  blockId: string;
  note?: string;
}

interface CreateChildTaskParams {
  parentId: string;
  data: CreateChildTaskRequest;
}

interface EnqueueTaskRunParams extends TaskIdParams {
  data?: EnqueueTaskRunRequest;
}

interface FanOutTaskRunsParams extends TaskIdParams {
  data: FanOutTaskRunsRequest;
}

function acknowledgeMutationSettlement(error: unknown): void {
  // mutateAsync callers own user-visible failure reporting. Settled handlers
  // only converge cached reads and intentionally run after either outcome.
  void error;
}

export function useCreateTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTaskRequest) => createTask(data),
    onSettled: () =>
      Promise.all([invalidateTaskQueries(queryClient), invalidateAggregateQueries(queryClient)]),
  });
}

export function useUpdateTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: UpdateTaskParams) => updateTask(id, data),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useDeleteTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => deleteTask(id),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      queryClient.removeQueries({ queryKey: tasksKeys.detail(id) });

      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function usePublishTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => publishTask(id),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useCancelTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: CancelTaskParams) => cancelTask(id, data ?? {}),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function usePauseTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: PauseTaskParams) => pauseTask(id, data),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useResumeTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: ResumeTaskParams) => resumeTask(id, data ?? {}),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useRecoverTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: RecoverTaskParams) => recoverTask(id, data ?? {}),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useApproveTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => approveTask(id),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useRejectTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => rejectTask(id),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useClearTaskBlock() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, blockId, note }: ClearTaskBlockParams) => clearTaskBlock(id, blockId, note),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useCreateChildTask() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ parentId, data }: CreateChildTaskParams) => createChildTask(parentId, data),
    onSettled: (_result, error, { parentId }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, parentId),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useEnqueueTaskRun() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: EnqueueTaskRunParams) => enqueueTaskRun(id, data ?? {}),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}

export function useFanOutTaskRuns() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: FanOutTaskRunsParams) => fanOutTaskRuns(id, data),
    onSettled: (_result, error, { id }) => {
      acknowledgeMutationSettlement(error);
      return Promise.all([
        invalidateTaskQueries(queryClient, id),
        invalidateAggregateQueries(queryClient),
      ]);
    },
  });
}
