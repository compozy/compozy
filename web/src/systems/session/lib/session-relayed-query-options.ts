import { queryOptions } from "@tanstack/react-query";

import { fetchSessionPendingWithDescendants } from "../adapters/session-api";
import { sessionKeys } from "./query-keys";
import { catalogStreamCoveredRefetchInterval } from "./session-catalog-stream-coverage";

const RELAYED_INTERACTIONS_REFETCH_INTERVAL_MS = 15_000;

/**
 * Asks reaching a main session from the sessions it started. A new ask needs no prompt to
 * show: every pending-ask change upserts the asking session on the catalog stream, whose
 * wake re-reads this list, so while that stream is live only the safety cadence remains.
 */
export function sessionRelayedInteractionsOptions(
  workspace: string,
  id: string,
  enabled: boolean,
  streamCovered = false
) {
  return queryOptions({
    queryKey: sessionKeys.interactions(workspace, id, "descendants"),
    queryFn: ({ signal }) => fetchSessionPendingWithDescendants(workspace, id, signal),
    enabled,
    refetchInterval: catalogStreamCoveredRefetchInterval(
      streamCovered,
      RELAYED_INTERACTIONS_REFETCH_INTERVAL_MS
    ),
    retry: false,
  });
}
