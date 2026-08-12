import { useQuery } from "@tanstack/react-query";

import {
  taskInspectOptions,
  taskRunDetailOptions,
  taskRunInspectOptions,
  taskTimelineOptions,
  taskTreeOptions,
} from "../lib/query-options";
import type { TaskTimelineFilter } from "../types";

interface QueryHookOptions {
  enabled?: boolean;
  refetchIntervalMs?: number | false;
}

function withQueryHookOptions<T extends object>(queryOptions: T, hookOptions: QueryHookOptions) {
  return {
    ...queryOptions,
    ...(hookOptions.refetchIntervalMs === undefined
      ? {}
      : { refetchInterval: hookOptions.refetchIntervalMs }),
  };
}

export function useTaskTimeline(
  id: string,
  filters: TaskTimelineFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(
    withQueryHookOptions(taskTimelineOptions(id, filters, options.enabled ?? true), options)
  );
}

export function useTaskTree(id: string, options: QueryHookOptions = {}) {
  return useQuery(withQueryHookOptions(taskTreeOptions(id, options.enabled ?? true), options));
}

export function useTaskInspect(id: string, options: QueryHookOptions = {}) {
  return useQuery(withQueryHookOptions(taskInspectOptions(id, options.enabled ?? true), options));
}

export function useTaskRunDetail(runId: string, options: QueryHookOptions = {}) {
  return useQuery(
    withQueryHookOptions(taskRunDetailOptions(runId, options.enabled ?? true), options)
  );
}

export function useTaskRunInspect(runId: string, options: QueryHookOptions = {}) {
  return useQuery(
    withQueryHookOptions(taskRunInspectOptions(runId, options.enabled ?? true), options)
  );
}
