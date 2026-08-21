import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import {
  taskDetailOptions,
  taskRunsOptions,
  tasksListOptions,
  withTaskProfileScope,
} from "../lib/query-options";
import { readTaskListData } from "../lib/task-list-query";
import type { TaskListFilter, TaskRunsFilter } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";
import { useProfileReadScope } from "@/systems/profiles";

export function useTasks(filters: TaskListFilter = {}, options: TaskQueryHookOptions = {}) {
  // Scope is applied at the one hook every task list goes through, so a switch
  // moves every consumer together and the key partitions by profile for free.
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery(
    withTaskQueryHookOptions(
      tasksListOptions(withTaskProfileScope(filters, params), options.enabled ?? true),
      options
    )
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
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(taskDetailOptions(id, params, options.enabled ?? true), options)
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
