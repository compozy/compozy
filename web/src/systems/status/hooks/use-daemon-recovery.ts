import { useQuery } from "@tanstack/react-query";

import { daemonRecovery, type DaemonRecovery } from "../lib/daemon-readiness";
import { statusOptions } from "../lib/query-options";
import type { StatusPayload } from "../types";

const RECOVERING_REFETCH_INTERVAL_MS = 2_000;
const SETTLED_REFETCH_INTERVAL_MS = 10_000;

function selectRecovery(status: StatusPayload): DaemonRecovery | null {
  return daemonRecovery(status.daemon);
}

function refetchInterval(query: { state: { data?: StatusPayload } }): number {
  const recovering = daemonRecovery(query.state.data?.daemon) !== null;
  return recovering ? RECOVERING_REFETCH_INTERVAL_MS : SETTLED_REFETCH_INTERVAL_MS;
}

/**
 * The runtime's session recovery after a restart, or null once it is fully ready. The shared
 * status query is read faster while recovery runs, so the notice clears soon after it ends.
 */
export function useDaemonRecovery(): DaemonRecovery | null {
  const query = useQuery({ ...statusOptions(), select: selectRecovery, refetchInterval });
  return query.data ?? null;
}
