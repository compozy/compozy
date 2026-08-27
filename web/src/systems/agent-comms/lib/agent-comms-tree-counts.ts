import { isNeedsYouCallState } from "./call-state";
import type { CallTreeGroup } from "./agent-comms-tree";

/** Per-tree counts. Absent facets mean the owner has not answered them yet. */
export interface CallTreeGroupCounts {
  total?: number;
  running?: number;
  needsYou?: number;
}

/**
 * Exact per-root counts from a complete loaded population, or the daemon's
 * scoped `total` when the Activity list is already filtered to one root.
 *
 * An incomplete workspace page must not be presented as each tree's total.
 */
export function countsForTreeGroups(
  groups: readonly CallTreeGroup[],
  options: { complete: boolean; scopedTotal?: number }
): ReadonlyMap<string, CallTreeGroupCounts> {
  const counts = new Map<string, CallTreeGroupCounts>();
  const scoped = options.scopedTotal !== undefined && groups.length === 1;

  for (const group of groups) {
    if (!options.complete) {
      if (scoped) counts.set(group.rootSessionId, { total: options.scopedTotal });
      continue;
    }

    let running = 0;
    let needsYou = 0;
    for (const row of group.rows) {
      if (row.state === "running") running += 1;
      if (row.state !== null && isNeedsYouCallState(row.state)) needsYou += 1;
    }
    counts.set(group.rootSessionId, {
      total: scoped ? options.scopedTotal : group.rows.length,
      running,
      needsYou,
    });
  }

  return counts;
}
