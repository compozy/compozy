import { queryOptions } from "@tanstack/react-query";

import { listNotificationPresets } from "../adapters/notifications-api";
import { notificationKeys } from "./query-keys";
import type { NotificationPresetFilter } from "../types";

const NOTIFICATION_COLLECTION_STALE_TIME = 15_000;
const NOTIFICATION_COLLECTION_REFETCH_INTERVAL = 45_000;
const NOTIFICATION_QUERY_RETRY_LIMIT = 2;

export function shouldRetryNotificationsQuery(failureCount: number): boolean {
  return failureCount < NOTIFICATION_QUERY_RETRY_LIMIT;
}

export function notificationPresetsOptions(filter: NotificationPresetFilter = {}) {
  return queryOptions({
    queryKey: notificationKeys.presetsList(filter),
    queryFn: ({ signal }) => listNotificationPresets(filter, signal),
    staleTime: NOTIFICATION_COLLECTION_STALE_TIME,
    refetchInterval: NOTIFICATION_COLLECTION_REFETCH_INTERVAL,
    retry: shouldRetryNotificationsQuery,
  });
}
