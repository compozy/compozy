import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { taskDetailOptions, taskRunsOptions, tasksListOptions } from "../lib/query-options";
import { readTaskListData } from "../lib/task-list-query";
import type { TaskListFilter, TaskRunsFilter } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";

export function useTasks(filters: TaskListFilter = {}, options: TaskQueryHookOptions = {}) {
  const query = useInfiniteQuery(
    withTaskQueryHookOptions(tasksListOptions(filters, options.enabled ?? true), options)
  );
  const catalog = readTaskListData(query.data);
  return {
    ...query,
    data: catalog.tasks,
    total: catalog.total,
    facets: catalog.facets,
  };
}

export function useTask(id: string, options: TaskQueryHookOptions = {}) {
  return useQuery(
    withTaskQueryHookOptions(taskDetailOptions(id, options.enabled ?? true), options)
  );
}

export function useTaskRuns(
  id: string,
  filters: TaskRunsFilter = {},
  options: TaskQueryHookOptions = {}
) {
  return useQuery(
    withTaskQueryHookOptions(taskRunsOptions(id, filters, options.enabled ?? true), options)
  );
}
