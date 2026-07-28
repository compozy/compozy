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
}

export function useTaskTimeline(
  id: string,
  filters: TaskTimelineFilter = {},
  options: QueryHookOptions = {}
) {
  return useQuery(taskTimelineOptions(id, filters, options.enabled ?? true));
}

export function useTaskTree(id: string, options: QueryHookOptions = {}) {
  return useQuery(taskTreeOptions(id, options.enabled ?? true));
}

export function useTaskInspect(id: string, options: QueryHookOptions = {}) {
  return useQuery(taskInspectOptions(id, options.enabled ?? true));
}

export function useTaskRunDetail(runId: string, options: QueryHookOptions = {}) {
  return useQuery(taskRunDetailOptions(runId, options.enabled ?? true));
}

export function useTaskRunInspect(runId: string, options: QueryHookOptions = {}) {
  return useQuery(taskRunInspectOptions(runId, options.enabled ?? true));
}
