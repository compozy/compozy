import { useMutation, useQueryClient } from "@tanstack/react-query";

import { archiveTask, dismissTask, markTaskRead } from "../adapters/tasks-api";
import { invalidateTriageQueries } from "./task-query-invalidation";

interface TaskIdParams {
  id: string;
}

function invalidateAfterTriageSettlement(
  error: unknown,
  invalidate: () => Promise<unknown>
): Promise<unknown> {
  // mutateAsync callers report failures; settled handlers converge cached triage reads.
  void error;
  return invalidate();
}

export function useMarkTaskRead() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => markTaskRead(id),
    onSettled: (_result, error, { id }) =>
      invalidateAfterTriageSettlement(error, () => invalidateTriageQueries(queryClient, id)),
  });
}

export function useArchiveTask() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => archiveTask(id),
    onSettled: (_result, error, { id }) =>
      invalidateAfterTriageSettlement(error, () => invalidateTriageQueries(queryClient, id)),
  });
}

export function useDismissTask() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id }: TaskIdParams) => dismissTask(id),
    onSettled: (_result, error, { id }) =>
      invalidateAfterTriageSettlement(error, () => invalidateTriageQueries(queryClient, id)),
  });
}
