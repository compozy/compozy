import { useQuery } from "@tanstack/react-query";

import { taskDashboardOptions } from "../lib/query-options";
import type { TaskDashboardFilter } from "../types";
import { type TaskQueryHookOptions, withTaskQueryHookOptions } from "./task-query-hook-options";
import { useProfileReadScope } from "@/systems/profiles";

export function useTaskDashboard(
  filters: TaskDashboardFilter = {},
  options: TaskQueryHookOptions = {}
) {
  // Scope applied at the one hook every consumer goes through, so a switch
  // moves them together and the key partitions by profile for free.
  const { params } = useProfileReadScope();
  return useQuery(
    withTaskQueryHookOptions(
      taskDashboardOptions({ ...filters, ...params }, options.enabled ?? true),
      options
    )
  );
}
