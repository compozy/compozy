import { useInfiniteQuery } from "@tanstack/react-query";

import { taskInboxBadgeOptions, taskInboxOptions } from "../lib/query-options";
import { readTaskInboxData } from "../lib/task-inbox-query";
import type { TaskInboxFilter } from "../types";
import { useProfileReadScope } from "@/systems/profiles";

interface QueryHookOptions {
  enabled?: boolean;
}

export function useTaskInbox(filters: TaskInboxFilter = {}, options: QueryHookOptions = {}) {
  // Scope applied at the one hook every consumer goes through, so a switch
  // moves them together and the key partitions by profile for free.
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery(
    taskInboxOptions({ ...filters, ...params }, options.enabled ?? true)
  );
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
