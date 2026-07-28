import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  cancelTaskRun,
  failTaskRun,
  forceFailTaskRun,
  forceReleaseTaskRun,
  retryTaskRun,
} from "../adapters/tasks-api";
import { tasksKeys } from "../lib/query-keys";
import type {
  CancelTaskRunRequest,
  FailTaskRunRequest,
  ForceFailTaskRunRequest,
  ForceReleaseTaskRunRequest,
  RetryTaskRunRequest,
} from "../types";
import { invalidateAggregateQueries, invalidateTaskQueries } from "./task-query-invalidation";

interface TaskRunIdParams {
  runId: string;
}

interface CancelTaskRunParams extends TaskRunIdParams {
  data?: CancelTaskRunRequest;
}

interface FailTaskRunParams extends TaskRunIdParams {
  data: FailTaskRunRequest;
}

interface ForceReleaseTaskRunParams extends TaskRunIdParams {
  data?: ForceReleaseTaskRunRequest;
}

interface ForceFailTaskRunParams extends TaskRunIdParams {
  data: ForceFailTaskRunRequest;
}

interface RetryTaskRunParams extends TaskRunIdParams {
  data?: RetryTaskRunRequest;
}

function invalidateAfterRunSettlement<T>(error: unknown, invalidate: () => T): T {
  // mutateAsync callers report failures; settled handlers converge cached run reads.
  void error;
  return invalidate();
}

export function useCancelTaskRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ runId, data }: CancelTaskRunParams) => cancelTaskRun(runId, data ?? {}),
    onSettled: (_result, error, { runId }) =>
      invalidateAfterRunSettlement(error, () =>
        Promise.all([
          queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(runId) }),
          invalidateTaskQueries(queryClient),
          invalidateAggregateQueries(queryClient),
        ])
      ),
  });
}

export function useFailTaskRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ runId, data }: FailTaskRunParams) => failTaskRun(runId, data),
    onSettled: (_result, error, { runId }) =>
      invalidateAfterRunSettlement(error, () =>
        Promise.all([
          queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(runId) }),
          invalidateTaskQueries(queryClient),
          invalidateAggregateQueries(queryClient),
        ])
      ),
  });
}

export function useForceReleaseTaskRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ runId, data }: ForceReleaseTaskRunParams) =>
      forceReleaseTaskRun(runId, data ?? {}),
    onSettled: (_result, error, { runId }) =>
      invalidateAfterRunSettlement(error, () =>
        Promise.all([
          queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(runId) }),
          invalidateTaskQueries(queryClient),
          invalidateAggregateQueries(queryClient),
        ])
      ),
  });
}

export function useForceFailTaskRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ runId, data }: ForceFailTaskRunParams) => forceFailTaskRun(runId, data),
    onSettled: (_result, error, { runId }) =>
      invalidateAfterRunSettlement(error, () =>
        Promise.all([
          queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(runId) }),
          invalidateTaskQueries(queryClient),
          invalidateAggregateQueries(queryClient),
        ])
      ),
  });
}

export function useRetryTaskRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ runId, data }: RetryTaskRunParams) => retryTaskRun(runId, data ?? {}),
    onSettled: (_result, error, { runId }) =>
      invalidateAfterRunSettlement(error, () =>
        Promise.all([
          queryClient.invalidateQueries({ queryKey: tasksKeys.runDetail(runId) }),
          invalidateTaskQueries(queryClient),
          invalidateAggregateQueries(queryClient),
        ])
      ),
  });
}
