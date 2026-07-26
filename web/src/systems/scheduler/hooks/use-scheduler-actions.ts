import { useMutation, useQueryClient } from "@tanstack/react-query";

import { drainScheduler, pauseScheduler, resumeScheduler } from "../adapters/scheduler-api";
import { schedulerKeys } from "../lib/query-keys";
import type {
  SchedulerDrainRequest,
  SchedulerPauseRequest,
  SchedulerResumeRequest,
} from "../types";

function invalidateSchedulerQueries(queryClient: ReturnType<typeof useQueryClient>) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: schedulerKeys.status() }),
    queryClient.invalidateQueries({ queryKey: schedulerKeys.all }),
  ]);
}

export function usePauseScheduler() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SchedulerPauseRequest = {}) => pauseScheduler(data),
    onSettled: () => invalidateSchedulerQueries(queryClient),
  });
}

export function useResumeScheduler() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SchedulerResumeRequest = {}) => resumeScheduler(data),
    onSettled: () => invalidateSchedulerQueries(queryClient),
  });
}

export function useDrainScheduler() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SchedulerDrainRequest = {}) => drainScheduler(data),
    onSettled: () => invalidateSchedulerQueries(queryClient),
  });
}
