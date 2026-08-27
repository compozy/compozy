/**
 * Whether each delegation child is parked, still working, or gone.
 *
 * The wire has no `child_state` today, and inventing parked/gone from session
 * stop reasons is a client guess. Until the daemon projects those states, this
 * hook returns an empty map so Activity pills stay absent rather than lying.
 */
import type { AgentCommsScope, ChildState } from "@/systems/agent-comms";

/** One rendered tree, and the children its rows named. */
export interface ActivityRootChildren {
  rootSessionId: string;
  childSessionIds: readonly string[];
}

const NO_CHILD_STATES: ReadonlyMap<string, ChildState> = new Map();

export function useActivityChildStates(
  _scope: AgentCommsScope,
  _roots: readonly ActivityRootChildren[],
  _live: boolean
): ReadonlyMap<string, ChildState> {
  return NO_CHILD_STATES;
}
