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
 * Per-tree counts come from the already-loaded tree rows, or from the daemon's
 * scoped `total` when Activity is already filtered to one root. Incomplete
 * workspace pages do not invent exact per-root totals.
 */
import { useNavigate } from "@tanstack/react-router";

import {
  countsForTreeGroups,
  isAgentCommsApiError,
  useAgentCommsActivity,
  useCallMutations,
} from "@/systems/agent-comms";
import { useSessionCatalogStreamStatus } from "@/systems/session";

import { useWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import type { AgentActivitySearch } from "./agent-activity-search";
import { useActivityChildStates } from "./use-activity-child-states";

export function useAgentsActivity(windowId: string, search: AgentActivitySearch = {}) {
  const live = useWindowLiveDataEnabled(windowId);
  const streamStatus = useSessionCatalogStreamStatus();
  const navigate = useNavigate({ from: "/agents" });
  const activity = useAgentCommsActivity({
    live,
    ...(search.root ? { rootSessionId: search.root } : {}),
  });
  const mutations = useCallMutations(activity.scope);

  const countsByRoot = countsForTreeGroups(activity.tree.groups, {
    complete: !activity.hasMore,
    ...(search.root && activity.total !== undefined ? { scopedTotal: activity.total } : {}),
  });
  const childStates = useActivityChildStates(
    activity.scope,
    activity.tree.groups.map(group => ({
      rootSessionId: group.rootSessionId,
      childSessionIds: group.rows.flatMap(row =>
        row.call.child_session_id ? [row.call.child_session_id] : []
      ),
    })),
    live
  );

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
    stopSubtree: (rootSessionId: string, profile: string) => {
      mutations.drainSubtree.mutate({
        sessionId: rootSessionId,
        reason: "stopped from Activity",
        profile,
      });
    },
    drainFailure: mutations.drainSubtree.isError
      ? {
          code: isAgentCommsApiError(mutations.drainSubtree.error)
            ? mutations.drainSubtree.error.code
            : null,
          message:
            mutations.drainSubtree.error instanceof Error
              ? mutations.drainSubtree.error.message
              : "The subtree stop failed.",
        }
      : null,
    drainOutcome: mutations.drainSubtree.data ?? null,
    retryStopSubtree: () => {
      if (mutations.drainSubtree.variables) {
        mutations.drainSubtree.mutate(mutations.drainSubtree.variables);
      }
    },
    pendingStopRootSessionId: mutations.drainSubtree.isPending
      ? (mutations.drainSubtree.variables?.sessionId ?? null)
      : null,
  };
}
