import { useQuery } from "@tanstack/react-query";

import { schedulerBacklogOptions, schedulerStatusOptions } from "../lib/query-options";
import type { SchedulerBacklogQuery } from "../types";

interface QueryHookOptions {
  enabled?: boolean;
}

export function useSchedulerStatus(options: QueryHookOptions = {}) {
  return useQuery(schedulerStatusOptions(options.enabled ?? true));
}

export function useSchedulerBacklog(
  query: SchedulerBacklogQuery = {},
  options: QueryHookOptions = {}
) {
  return useQuery(schedulerBacklogOptions(query, options.enabled ?? true));
}
