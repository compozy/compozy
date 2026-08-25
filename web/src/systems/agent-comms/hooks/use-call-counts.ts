/**
 * Daemon counts for filtered call populations.
 *
 * Each count is a `limit=1` read whose only product is `total`. That is the
 * summary projection: `listCalls` counts the filtered set before it paginates,
 * so one row of wire cost buys an authoritative number that does not drift as
 * the operator pages.
 *
 * `useCallCounts` takes several filters at once because a single surface usually
 * needs several facets — a tree header wants its total, its running count, and
 * its needs-you count — and issuing them as one hook keeps their enabled state
 * and polling cadence identical instead of three hooks drifting apart.
 */
import { useQueries, useQuery } from "@tanstack/react-query";

import type { CallCountFilter } from "../adapters/agent-comms-api";
import { callCountOptions } from "../lib/query-options";
import type { AgentCommsScope } from "../lib/agent-comms-scope";

export type { CallCountFilter };

export interface UseCallCountOptions {
  live?: boolean;
  enabled?: boolean;
}

/** One count. Undefined until the daemon answers — never zero as a placeholder. */
export function useCallCount(
  scope: AgentCommsScope,
  filter: CallCountFilter,
  { live = false, enabled = true }: UseCallCountOptions = {}
): number | undefined {
  const query = useQuery(callCountOptions(scope, filter, live, enabled));
  return query.data?.total;
}

/**
 * Several counts, in one hook.
 *
 * Returns them positionally so the caller keeps its own names for the facets,
 * and returns `undefined` per slot until that facet answers — a half-loaded
 * summary shows the clauses it has rather than blocking on the slowest.
 */
export function useCallCounts(
  scope: AgentCommsScope,
  filters: readonly CallCountFilter[],
  { live = false, enabled = true }: UseCallCountOptions = {}
): (number | undefined)[] {
  return useQueries({
    queries: filters.map(filter => callCountOptions(scope, filter, live, enabled)),
    combine: results => results.map(result => result.data?.total),
  });
}
