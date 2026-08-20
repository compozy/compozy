import type { HomeOverview } from "../types";

/** The pulse spans a wider window than today/outcomes, so it can be the only witness. */
function hasNoPulse(overview: HomeOverview): boolean {
  const { buckets, busiest, longest_session } = overview.pulse;
  if (busiest || longest_session) return false;
  return buckets.every(bucket => bucket.events === 0);
}

/**
 * True only when the overview reports no work at all — every counter it carries
 * is zero and nothing is running right now. `has_live_work` and the pulse are
 * part of the test on purpose: a fresh install with an agent already working, or
 * with activity outside the today/outcomes windows, has zero completed counters,
 * and telling that person nothing has happened would be untrue.
 */
export function hasNoRecordedWork(overview: HomeOverview): boolean {
  return (
    hasNoPulse(overview) &&
    overview.attention.total === 0 &&
    overview.today.runs_completed === 0 &&
    overview.today.runs_failed === 0 &&
    overview.today.tasks_closed === 0 &&
    overview.outcomes.completed === 0 &&
    overview.outcomes.failed === 0 &&
    overview.outcomes.canceled === 0 &&
    overview.usage.total_tokens === 0 &&
    overview.network.messages_today === 0 &&
    overview.system.hook_runs_today === 0 &&
    overview.system.hook_failures_today === 0 &&
    !overview.freshness.has_live_work
  );
}
