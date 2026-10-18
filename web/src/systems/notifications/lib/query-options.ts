import { queryOptions } from "@tanstack/react-query";

import { listAttentionNotifications } from "../adapters/attention-api";

import { notificationKeys } from "./query-keys";
const NOTIFICATION_QUERY_RETRY_LIMIT = 2;

export function shouldRetryNotificationsQuery(failureCount: number): boolean {
  return failureCount < NOTIFICATION_QUERY_RETRY_LIMIT;
}

export function attentionNotificationsOptions(profile: string) {
  return queryOptions({
    queryKey: notificationKeys.attention(profile),
    queryFn: ({ signal }) => listAttentionNotifications(profile, signal),
    staleTime: 0,
    retry: shouldRetryNotificationsQuery,
  });
}
