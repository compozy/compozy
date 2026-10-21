import { queryOptions } from "@tanstack/react-query";

import { fetchSessionMonitors } from "../adapters/session-monitor-api";
import { sessionKeys } from "./query-keys";

/**
 * The session's monitors. Every monitor start, cancel, or delivery upserts the
 * session on the catalog stream, which invalidates this read.
 */
export function sessionMonitorsOptions(workspaceId: string, sessionId: string) {
  return queryOptions({
    queryKey: sessionKeys.monitors(workspaceId, sessionId),
    queryFn: ({ signal }) => fetchSessionMonitors(workspaceId, sessionId, signal),
    enabled: workspaceId !== "" && sessionId !== "",
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
