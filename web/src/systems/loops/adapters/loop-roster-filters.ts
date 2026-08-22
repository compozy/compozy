/**
 * The `state` values the roster route accepts as a FILTER.
 *
 * Deliberately narrower than the roster's own state projection: `pending` is an
 * output state only, so asking for it is a `400 invalid_node_state` (peer review
 * B-007, UT-050).
 *
 * This is wire vocabulary, so it is declared in the adapter layer and nowhere
 * else. `lib/loop-run-state-copy` re-exports it for the UI, and the MSW
 * resolvers validate against this same array — one list, three readers. It was
 * previously declared in `lib`, which meant the adapter had to import upward
 * into a module carrying icon and token dependencies, against the repo's
 * `adapters → lib → hooks → components` flow.
 */
export const LOOP_ROSTER_STATE_FILTERS = [
  "all",
  "running",
  "queued",
  "waiting",
  "retrying",
  "paused",
  "quarantined",
  "succeeded",
  "failed",
  "canceled",
  "not_taken",
] as const;

export type LoopRosterStateFilter = (typeof LOOP_ROSTER_STATE_FILTERS)[number];

/** Whether the daemon would accept this value as a roster `state` filter. */
export function isLoopRosterStateFilter(value: string): value is LoopRosterStateFilter {
  return LOOP_ROSTER_STATE_FILTERS.some(filter => filter === value);
}
