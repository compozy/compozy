import { useInfiniteQuery } from "@tanstack/react-query";

import { taskInboxBadgeOptions, taskInboxOptions } from "../lib/query-options";
import { readTaskInboxData } from "../lib/task-inbox-query";
import type { TaskInboxFilter } from "../types";

interface QueryHookOptions {
  enabled?: boolean;
}

export function useTaskInbox(filters: TaskInboxFilter = {}, options: QueryHookOptions = {}) {
  const query = useInfiniteQuery(taskInboxOptions(filters, options.enabled ?? true));
  return {
    ...query,
    data: readTaskInboxData(query.data),
  };
}

export function useTaskInboxBadge(filters: TaskInboxFilter = {}, options: QueryHookOptions = {}) {
  const query = useInfiniteQuery(taskInboxBadgeOptions(filters, options.enabled ?? true));
  return {
    ...query,
    data: readTaskInboxData(query.data),
  };
}
