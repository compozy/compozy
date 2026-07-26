import type { QueryClient } from "@tanstack/react-query";

import { schedulerKeys } from "@/systems/scheduler";

import { tasksKeys } from "../lib/query-keys";

export function invalidateTaskQueries(queryClient: QueryClient, id?: string) {
  const pending = [
    queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.runsRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.timelineRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.treeRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.runDetails() }),
  ];

  if (id) {
    pending.push(queryClient.invalidateQueries({ queryKey: tasksKeys.detail(id) }));
  }

  return Promise.all(pending);
}

export function invalidateAggregateQueries(queryClient: QueryClient) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: tasksKeys.dashboardRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.inboxRoot() }),
    queryClient.invalidateQueries({ queryKey: schedulerKeys.all }),
  ]);
}

export function invalidateTriageQueries(queryClient: QueryClient, id?: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: tasksKeys.triageRoot() }),
    queryClient.invalidateQueries({ queryKey: tasksKeys.lists() }),
    ...(id ? [queryClient.invalidateQueries({ queryKey: tasksKeys.detail(id) })] : []),
    queryClient.invalidateQueries({ queryKey: tasksKeys.inboxRoot() }),
  ]);
}
