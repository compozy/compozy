import { useQuery } from "@tanstack/react-query";

import { daemonRecovery, type DaemonRecovery } from "../lib/daemon-readiness";
import { statusOptions } from "../lib/query-options";
import type { StatusPayload } from "../types";

const RECOVERING_REFETCH_INTERVAL_MS = 2_000;
const SETTLED_REFETCH_INTERVAL_MS = 10_000;

function selectRecovery(status: StatusPayload): DaemonRecovery | null {
  return daemonRecovery(status.daemon);
}

/** The shared status query is read faster while the runtime recovers. */
export function recoveryRefetchInterval(query: { state: { data?: StatusPayload } }): number {
  const recovering = daemonRecovery(query.state.data?.daemon) !== null;
  return recovering ? RECOVERING_REFETCH_INTERVAL_MS : SETTLED_REFETCH_INTERVAL_MS;
}

/**
 * The runtime's session recovery after a restart, or null once it is fully ready. The shared
 * status query is read faster while recovery runs, so the notice clears soon after it ends.
 * A failed status read says nothing about the runtime's phase, so the last answer's notice is
 * hidden until the next successful response.
 */
export function useDaemonRecovery(): DaemonRecovery | null {
  const query = useQuery({
    ...statusOptions(),
    select: selectRecovery,
    refetchInterval: recoveryRefetchInterval,
  });
  if (query.isError) return null;
  return query.data ?? null;
}
