import { useQuery } from "@tanstack/react-query";

import {
  taskInspectOptions,
  taskRunDetailOptions,
  taskRunInspectOptions,
  taskTimelineOptions,
  taskTreeOptions,
} from "../lib/query-options";
import type { TaskTimelineFilter } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";

export function useTaskTimeline(
  id: string,
  filters: TaskTimelineFilter = {},
  options: TaskQueryHookOptions = {}
) {
  return useQuery(
    withTaskQueryHookOptions(taskTimelineOptions(id, filters, options.enabled ?? true), options)
  );
}

export function useTaskTree(id: string, options: TaskQueryHookOptions = {}) {
  return useQuery(withTaskQueryHookOptions(taskTreeOptions(id, options.enabled ?? true), options));
}

export function useTaskInspect(id: string, options: TaskQueryHookOptions = {}) {
  return useQuery(
    withTaskQueryHookOptions(taskInspectOptions(id, options.enabled ?? true), options)
  );
}

export function useTaskRunDetail(runId: string, options: TaskQueryHookOptions = {}) {
  return useQuery(
    withTaskQueryHookOptions(taskRunDetailOptions(runId, options.enabled ?? true), options)
  );
}

export function useTaskRunInspect(runId: string, options: TaskQueryHookOptions = {}) {
  return useQuery(
    withTaskQueryHookOptions(taskRunInspectOptions(runId, options.enabled ?? true), options)
  );
}
