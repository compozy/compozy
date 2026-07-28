import { queryOptions } from "@tanstack/react-query";

import { getScheduler, getSchedulerBacklog } from "../adapters/scheduler-api";
import type { SchedulerBacklogQuery } from "../types";
import { schedulerKeys } from "./query-keys";

export function schedulerStatusOptions(enabled = true) {
  return queryOptions({
    enabled,
    queryKey: schedulerKeys.status(),
    queryFn: ({ signal }) => getScheduler(signal),
    staleTime: 15_000,
  });
}

export function schedulerBacklogOptions(query: SchedulerBacklogQuery = {}, enabled = true) {
  return queryOptions({
    enabled,
    queryKey: schedulerKeys.backlog(query),
    queryFn: ({ signal }) => getSchedulerBacklog(query, signal),
    staleTime: 15_000,
  });
}
