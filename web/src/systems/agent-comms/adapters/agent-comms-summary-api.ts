/**
 * The daemon's summary projection for calls.
 *
 * There is no `/calls/summary` route and none is needed: `listCalls` counts
 * before it paginates. `global_db_calls_read.go` builds one `where` clause from
 * the read scope plus the filters, runs `SELECT COUNT(*)` against it, and only
 * afterwards appends the cursor predicate and `LIMIT`. So `total` describes the
 * whole filtered population no matter where the operator sits in the cursor.
 *
 * A counted request at `limit=1` is therefore the summary projection: one row of
 * wire cost for an authoritative count. Every count in this feature — the dock
 * badge, per-tree coalescing, tree root summaries, the Calls panel's two
 * directions, roster instance counts — is one of these probes under its own
 * filter. Nothing is ever counted from a loaded page.
 */
import type { ProfileScopeParams } from "@/systems/profiles";

import { listCalls, type CallsListFilter } from "./agent-comms-calls-api";

/** Cheapest page that still carries the count. */
const COUNT_PROBE_LIMIT = 1;

/** A count that knows what it counted. */
export interface CallCount {
  total: number;
}

/**
 * A count is a property of a population, not of a page within it, so paging
 * inputs are not part of the filter.
 */
export type CallCountFilter = Omit<CallsListFilter, "cursor" | "limit">;

/**
 * Count one exactly-filtered call population.
 *
 * `cursor` and `limit` are deliberately not part of the filter type here — a
 * count is a property of the population, not of a page within it.
 */
export async function countCalls(
  workspaceId: string,
  filter: CallCountFilter,
  scope: ProfileScopeParams,
  signal?: AbortSignal
): Promise<CallCount> {
  const page = await listCalls(workspaceId, { ...filter, limit: COUNT_PROBE_LIMIT }, scope, signal);
  return { total: page.total };
}
