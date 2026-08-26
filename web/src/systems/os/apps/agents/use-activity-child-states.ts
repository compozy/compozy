/**
 * Whether each delegation child is parked, still working, or gone.
 *
 * A call record names its child but says nothing about it: there is no
 * `child_state` on the wire, and the daemon's `parked_at` never leaves the
 * store. The only public evidence is the child session itself, so Activity
 * reads the session catalog for each tree it renders and projects the answer.
 *
 * Three things about this read are deliberate:
 *
 * - **`useQueries`, never a loop of `useSessions`.** One tree is one query and
 *   the tree list is dynamic, so calling a hook per root would break the rules
 *   of hooks the moment a tree appears or disappears.
 * - **Scope is explicit in every filter.** `useSessions` sends
 *   `all_workspaces: true` whenever its workspace argument is empty, which here
 *   would read every workspace's sessions to decorate one workspace's tree. The
 *   workspace id and the profile params are passed by hand instead, so scope
 *   lives in the request and in the cache key rather than in a default.
 * - **Fail open, per root.** A root whose catalog is still loading or has
 *   errored contributes no entries at all. `gone` is a claim that a child is
 *   really absent, and a claim like that needs a complete catalog behind it —
 *   never a slow network.
 */
import { useQueries } from "@tanstack/react-query";

import {
  childStatesForRoot,
  LIVE_CALL_POLL_INTERVAL,
  type AgentCommsScope,
  type ChildState,
} from "@/systems/agent-comms";
import { sessionsCompleteListOptions } from "@/systems/session";

/** One rendered tree, and the children its rows named. */
export interface ActivityRootChildren {
  rootSessionId: string;
  childSessionIds: readonly string[];
}

export function useActivityChildStates(
  scope: AgentCommsScope,
  roots: readonly ActivityRootChildren[],
  live: boolean
): ReadonlyMap<string, ChildState> {
  const ready = scope.workspaceId !== "";
  return useQueries({
    queries: roots.map(root => ({
      ...sessionsCompleteListOptions({
        workspace_id: scope.workspaceId,
        root: root.rootSessionId,
        ...scope.params,
      }),
      enabled: ready,
      refetchInterval: live ? LIVE_CALL_POLL_INTERVAL : (false as const),
    })),
    combine: results => {
      const merged = new Map<string, ChildState>();
      results.forEach((result, index) => {
        const expected = roots[index]?.childSessionIds ?? [];
        // `undefined` unless this root's catalog is complete and healthy — the
        // projection then says nothing rather than guessing.
        const sessions = result.isSuccess ? result.data.sessions : undefined;
        for (const [childId, state] of childStatesForRoot(expected, sessions)) {
          merged.set(childId, state);
        }
      });
      return merged;
    },
  });
}
