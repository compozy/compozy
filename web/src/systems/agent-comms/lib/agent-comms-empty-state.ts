/**
 * Which of the four surface states a calls pane is in, and what an empty one
 * should say.
 *
 * The resolution itself is `@compozy/ui`'s `resolveDataSurfaceState` — the same
 * precedence every listing in the app uses (loading wins, then error, then
 * empty). What this module adds is the domain question underneath: *why* is it
 * empty? "No calls match this filter" and "no agent has ever delegated here" are
 * different situations and want different copy, and only the caller knows which
 * filters were applied.
 *
 * Emptiness is decided from the daemon's `total`, not from how many rows the
 * page happened to return. A first page can be short for reasons that have
 * nothing to do with the population being empty.
 */
import { resolveDataSurfaceState, type DataSurfaceState } from "@compozy/ui";

export interface CallSurfaceStateInput {
  isLoading: boolean;
  error: Error | null;
  /** The daemon's count for this filtered population. */
  total: number | undefined;
  /** True when any filter beyond the surface's own scope is active. */
  filtered: boolean;
}

export type CallEmptyReason =
  /** Nothing has ever been delegated in this scope — teach the feature. */
  | "no-calls"
  /** Calls exist, just none matching the current filter — offer to clear it. */
  | "no-matches";

export interface CallSurfaceState {
  status: DataSurfaceState;
  /** Set only when `status` is `empty`. */
  emptyReason: CallEmptyReason | null;
}

export function resolveCallSurfaceState(input: CallSurfaceStateInput): CallSurfaceState {
  const status = resolveDataSurfaceState({
    isLoading: input.isLoading,
    error: input.error,
    isEmpty: input.total === 0,
  });
  if (status !== "empty") return { status, emptyReason: null };
  return { status, emptyReason: input.filtered ? "no-matches" : "no-calls" };
}
