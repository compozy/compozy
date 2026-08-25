/**
 * The Activity location's view model.
 *
 * Liveness here is a poll, not a stream, and that is a fact about the runtime
 * rather than a shortcut: calls emit server-side extension hooks
 * (`internal/calls/hooks.go`), which are not reachable from a browser. There is
 * no calls SSE endpoint to subscribe to, so the honest implementation refreshes
 * on an interval and *says so* when the shell's own live connection drops.
 *
 * `live` and `stale` arrive as inputs rather than being read here. The shell
 * already owns both signals — window visibility and the session catalog stream's
 * connection status — and that stream's hook owns its socket: calling it a
 * second time would let this view's unmount close the shell's connection. So the
 * location passes what it knows, exactly as the Network app passes its own live
 * flag down through a context it owns.
 */
import { useInfiniteQuery } from "@tanstack/react-query";

import { buildCallTree, type CallCommsTree } from "../lib/agent-comms-tree";
import { resolveCallSurfaceState, type CallSurfaceState } from "../lib/agent-comms-empty-state";
import { callsListOptions } from "../lib/query-options";
import { useAgentCommsScope } from "./use-agent-comms-scope";
import type { AgentCommsScope } from "../lib/agent-comms-scope";
import type { CallPayload } from "../types";

export interface AgentCommsActivityModel {
  scope: AgentCommsScope;
  calls: CallPayload[];
  tree: CallCommsTree;
  /** The daemon's count for the whole filtered population. */
  total: number | undefined;
  surface: CallSurfaceState;
  hasMore: boolean;
  loadingMore: boolean;
  loadMore: () => void;
  refetch: () => void;
}

export interface UseAgentCommsActivityOptions {
  /** The window is visible and should keep data fresh. */
  live: boolean;
  /** Narrow to one delegation tree. Absent means the whole workspace. */
  rootSessionId?: string;
  /** Narrow to a state subset, comma-separated as the daemon expects. */
  state?: string;
}

export function useAgentCommsActivity(
  options: UseAgentCommsActivityOptions
): AgentCommsActivityModel {
  const scope = useAgentCommsScope();

  const filter = {
    ...(options.rootSessionId ? { root_session_id: options.rootSessionId } : {}),
    ...(options.state ? { state: options.state } : {}),
  };

  const query = useInfiniteQuery(callsListOptions(scope, filter, options.live));

  const calls = (query.data?.pages ?? []).flatMap(page => page.items);
  const tree = buildCallTree(calls);
  // `total` rides on every page, so the first one already carries the whole
  // population's count.
  const total = query.data?.pages[0]?.total;

  return {
    scope,
    calls,
    tree,
    total,
    surface: resolveCallSurfaceState({
      isLoading: query.isPending,
      error: query.error,
      total,
      filtered: Boolean(options.rootSessionId || options.state),
    }),
    hasMore: query.hasNextPage,
    loadingMore: query.isFetchingNextPage,
    loadMore: () => {
      void query.fetchNextPage();
    },
    refetch: () => {
      void query.refetch();
    },
  };
}
