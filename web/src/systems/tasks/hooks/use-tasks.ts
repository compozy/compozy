import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { taskDetailOptions, taskRunsOptions, tasksListOptions } from "../lib/query-options";
import { readTaskListData } from "../lib/task-list-query";
import type { TaskListFilter, TaskRunsFilter } from "../types";

interface QueryHookOptions {
  enabled?: boolean;
  refetchIntervalMs?: number | false;
}

export function useTasks(filters: TaskListFilter = {}, options: QueryHookOptions = {}) {
  const query = useInfiniteQuery({
    ...tasksListOptions(filters, options.enabled ?? true),
    ...(options.refetchIntervalMs === undefined
      ? {}
      : { refetchInterval: options.refetchIntervalMs }),
  });
  const catalog = readTaskListData(query.data);
  return {
    ...query,
    data: catalog.tasks,
    total: catalog.total,
    facets: catalog.facets,
  };
}

export function useTask(id: string, options: QueryHookOptions = {}) {
  return useQuery(taskDetailOptions(id, options.enabled ?? true));
}

export function useTaskRuns(
  id: string,
  filters: TaskRunsFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(taskRunsOptions(id, filters, options.enabled ?? true));
}
