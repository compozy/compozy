/**
 * Read specific call records by id.
 *
 * The transcript names which calls a turn made; this fetches what they became.
 * Reading live rather than trusting the transcript's snapshot matters: a tool
 * result was written the moment the call was accepted, so its state there is
 * forever `running`. The card has to show what the call *is*.
 *
 * Each id is its own query, so these share the cache with the call-detail
 * location — opening a call the transcript already showed is instant, and a
 * settled call is fetched once.
 */
import { useQueries } from "@tanstack/react-query";

import { callDetailOptions } from "../lib/query-options";
import { useAgentCommsScope } from "./use-agent-comms-scope";
import type { CallPayload } from "../types";

export interface CallsByIdModel {
  /** Records that resolved, in the order their ids were given. */
  calls: CallPayload[];
  loading: boolean;
}

export function useCallsById(callIds: readonly string[], live = false): CallsByIdModel {
  const scope = useAgentCommsScope();
  return useQueries({
    queries: callIds.map(callId => callDetailOptions(scope, callId, live)),
    combine: results => ({
      calls: results
        .map(result => result.data)
        .filter((call): call is CallPayload => call !== undefined),
      loading: results.some(result => result.isPending),
    }),
  });
}
