import { keepPreviousData, queryOptions } from "@tanstack/react-query";

import {
  fetchSessionHistoryCleanupRun,
  previewSessionHistoryCleanup,
  type SessionHistoryCleanupRequest,
} from "../adapters/session-history-cleanup-api";
import { sessionKeys } from "./query-keys";

/**
 * The dry run behind the cleanup dialog's live count. Each change of the checks or the range is
 * its own read; the previous count stays on screen until the next one lands.
 */
export function sessionHistoryCleanupPreviewOptions(
  profile: string,
  request: SessionHistoryCleanupRequest
) {
  return queryOptions({
    queryKey: sessionKeys.historyCleanupPreview(profile, request),
    queryFn: ({ signal }) => previewSessionHistoryCleanup(profile, request, signal),
    placeholderData: keepPreviousData,
    staleTime: 0,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/**
 * The profile's running or most recent cleanup. The run publishes `history_cleanup` on the
 * catalog stream as it progresses and when it ends, which invalidates this read.
 */
export function sessionHistoryCleanupRunOptions(profile: string) {
  return queryOptions({
    queryKey: sessionKeys.historyCleanupRun(profile),
    queryFn: ({ signal }) => fetchSessionHistoryCleanupRun(profile, signal),
    staleTime: 30_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
