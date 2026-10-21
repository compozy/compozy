import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { cancelSessionMonitor, type SessionMonitorPayload } from "../adapters/session-monitor-api";
import { sessionKeys } from "../lib/query-keys";
import { sessionMonitorsOptions } from "../lib/session-monitor-query-options";

export interface SessionMonitorsModel {
  /** Monitors still waiting to wake the session, newest first. */
  monitors: readonly SessionMonitorPayload[];
  loading: boolean;
  failed: boolean;
  retry: () => void;
  /** Cancels one monitor; rejects so the row can report the failure. */
  cancel: (monitorId: string) => Promise<unknown>;
  /** The monitor whose cancel is in flight. */
  cancelingId: string | null;
}

const WAITING_STATUSES = new Set(["active", "delivering"]);

/** The composer's monitor list, read only while `enabled` (the popover is open). */
export function useSessionMonitors(
  workspaceId: string,
  sessionId: string,
  enabled: boolean
): SessionMonitorsModel {
  const queryClient = useQueryClient();
  const options = sessionMonitorsOptions(workspaceId, sessionId);
  const queryEnabled = options.enabled === true && enabled;
  const query = useQuery({ ...options, enabled: queryEnabled });
  const mutation = useMutation({
    mutationFn: (monitorId: string) => cancelSessionMonitor(workspaceId, sessionId, monitorId),
    onSettled: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: sessionKeys.monitors(workspaceId, sessionId) }),
        queryClient.invalidateQueries({
          queryKey: sessionKeys.detail(workspaceId, sessionId),
          exact: true,
        }),
      ]),
  });
  return {
    monitors: (query.data?.monitors ?? []).filter(item => WAITING_STATUSES.has(item.status)),
    loading: query.isPending && queryEnabled,
    failed: query.isError,
    retry: () => void query.refetch(),
    cancel: monitorId => mutation.mutateAsync(monitorId),
    cancelingId: mutation.isPending ? (mutation.variables ?? null) : null,
  };
}
