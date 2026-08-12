import { useQuery } from "@tanstack/react-query";

import { taskDashboardOptions } from "../lib/query-options";
import type { TaskDashboardFilter } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";

export function useTaskDashboard(
  filters: TaskDashboardFilter = {},
  options: TaskQueryHookOptions = {}
) {
  return useQuery(
    withTaskQueryHookOptions(taskDashboardOptions(filters, options.enabled ?? true), options)
  );
}
