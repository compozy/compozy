import { queryOptions } from "@tanstack/react-query";

import { fetchSessionPendingWithDescendants } from "../adapters/session-api";
import { sessionKeys } from "./query-keys";

/** Asks reaching a main session from the sessions it started. A new ask needs no prompt to show. */
export function sessionRelayedInteractionsOptions(workspace: string, id: string, enabled: boolean) {
  return queryOptions({
    queryKey: sessionKeys.interactions(workspace, id, "descendants"),
    queryFn: ({ signal }) => fetchSessionPendingWithDescendants(workspace, id, signal),
    enabled,
    refetchInterval: 15_000,
    retry: false,
  });
}
