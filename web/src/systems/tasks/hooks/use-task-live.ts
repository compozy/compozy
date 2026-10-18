import { useQuery } from "@tanstack/react-query";

import {
  taskInspectOptions,
  taskRunDetailOptions,
  taskRunInspectOptions,
  taskTimelineOptions,
  taskTreeOptions,
  withTaskProfileScope,
} from "../lib/query-options";
import type { TaskTimelineFilter } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";
import { useProfileReadScope } from "@/systems/profiles";

export function useTaskTimeline(
  id: string,
  filters: TaskTimelineFilter = {},
  options: TaskQueryHookOptions = {}
) {
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(
      taskTimelineOptions(id, withTaskProfileScope(filters, params), options.enabled ?? true),
      options
    )
  );
}

export function useTaskTree(id: string, options: TaskQueryHookOptions = {}) {
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(taskTreeOptions(id, params, options.enabled ?? true), options)
  );
}

export function useTaskInspect(id: string, options: TaskQueryHookOptions = {}) {
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(taskInspectOptions(id, params, options.enabled ?? true), options)
  );
}

export function useTaskRunDetail(runId: string, options: TaskQueryHookOptions = {}) {
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(taskRunDetailOptions(runId, params, options.enabled ?? true), options)
  );
}

export function useTaskRunInspect(runId: string, options: TaskQueryHookOptions = {}) {
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(taskRunInspectOptions(runId, params, options.enabled ?? true), options)
  );
}
