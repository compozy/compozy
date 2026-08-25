/**
 * View model for the Activity location.
 *
 * This hook is the seam between the shell and the agent-comms system. It reads
 * the two things only the shell knows — whether this window is live, and whether
 * the session catalog stream is connected — and hands them down, rather than
 * letting the system reach for them. That keeps `useSessionCatalogStreams`
 * single-owner: the shell already holds that socket, and a second subscriber's
 * unmount would close it.
 *
 * It also issues the per-tree count probes. Each is a `limit=1` filtered read
 * whose only product is the daemon's `total`, so a tree header can say "3 calls ·
 * 2 running" from real numbers instead of counting the rows on screen.
 */
import { useNavigate } from "@tanstack/react-router";

import {
  useAgentCommsActivity,
  useCallCounts,
  useCallMutations,
  type CallTreeGroupCounts,
} from "@/systems/agent-comms";
import { useSessionCatalogStreamStatus } from "@/systems/session";

import { useWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { useActivityChildStates } from "./use-activity-child-states";

/** Facets each rendered tree asks the daemon for, in probe order. */
const TREE_FACETS = ["total", "running", "needsYou"] as const;

export function useAgentsActivity(windowId: string) {
  const live = useWindowLiveDataEnabled(windowId);
  const streamStatus = useSessionCatalogStreamStatus();
  const navigate = useNavigate({ from: "/agents" });
  const activity = useAgentCommsActivity({ live });
  const mutations = useCallMutations(activity.scope);

  const rootSessionIds = activity.tree.groups.map(group => group.rootSessionId);

  // One probe per rendered tree per facet. They key separately from the row
  // pages, so refreshing a header never evicts the rows underneath it.
  const countFilters = rootSessionIds.flatMap(rootSessionId => [
    { root_session_id: rootSessionId },
    { root_session_id: rootSessionId, state: "running" },
    { root_session_id: rootSessionId, attention: true },
  ]);
  const counts = useCallCounts(activity.scope, countFilters, { live });

  const countsByRoot = new Map<string, CallTreeGroupCounts>();
  rootSessionIds.forEach((rootSessionId, index) => {
    const base = index * TREE_FACETS.length;
    countsByRoot.set(rootSessionId, {
      total: counts[base],
      running: counts[base + 1],
      needsYou: counts[base + 2],
    });
  });

  // Each tree names the children its rows point at, so the catalog read knows
  // who to expect — which is the only way a child that is *missing* can be
  // reported as gone rather than silently omitted.
  const rootChildren = activity.tree.groups.map(group => {
    const childSessionIds: string[] = [];
    for (const row of group.rows) {
      const childId = row.call.child_session_id ?? "";
      if (childId !== "") childSessionIds.push(childId);
    }
    return { rootSessionId: group.rootSessionId, childSessionIds };
  });
  const childStates = useActivityChildStates(activity.scope, rootChildren, live);

  return {
    ...activity,
    countsByRoot,
    childStates,
    /** The stream is down: rows stay, counts stop. */
    stale: streamStatus === "stale",
    openCall: (callId: string) => {
      void navigate({ to: "/agents/calls/$callId", params: { callId } });
    },
    openCatalog: () => {
      void navigate({ to: "/agents" });
    },
    stopSubtree: (rootSessionId: string) => {
      mutations.drainSubtree.mutate({
        sessionId: rootSessionId,
        reason: "stopped from Activity",
      });
    },
    pendingStopRootSessionId: mutations.drainSubtree.isPending
      ? (mutations.drainSubtree.variables?.sessionId ?? null)
      : null,
  };
}
