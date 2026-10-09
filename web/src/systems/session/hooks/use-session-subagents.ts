import { useInfiniteQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useSyncExternalStore } from "react";

import { cancelSubagent } from "../adapters/subagent-api";
import type { SubagentView } from "../components/subagents/types";
import { sessionKeys } from "../lib/query-keys";
import { subagentViewFromPayload } from "../lib/subagent-payload";
import { SUBAGENT_LIST_PAGE_SIZE, sessionSubagentsOptions } from "../lib/subagent-query-options";
import type { SubagentRoster } from "../lib/subagent-roster";

export interface SessionSubagentsModel {
  subagents: readonly SubagentView[];
  /** Cancels one delegated subagent; rejects so the row raises its stop-failed toast. */
  stop: (subagent: SubagentView) => Promise<unknown>;
}

export interface SessionSubagentsOptions {
  /** Read nothing while false (a closed hover preview). */
  enabled?: boolean;
  /**
   * Walk every list page. The inspector roster pins failures across the whole
   * set; a five-row preview reads one page.
   */
  allPages?: boolean;
}

/**
 * The parent's stream-fed roster, only while it is live: the parent's thread is
 * mounted and observing it (that thread owns the session stream writing it), a
 * snapshot arrived, and no row awaits reconnect confirmation. It is read
 * without adding an observer, so the observer count is exactly the threads
 * holding the stream. The snapshot holds the newest 200 rows, so a full
 * snapshot defers to the list route.
 */
function useLiveSubagentRoster(
  workspaceId: string,
  sessionId: string,
  enabled: boolean
): SubagentRoster | null {
  const cache = useQueryClient().getQueryCache();
  const queryKey = sessionKeys.subagentRoster(workspaceId, sessionId);
  const subscribe = (notify: () => void) => cache.subscribe(notify);
  const find = () => cache.find<SubagentRoster>({ queryKey, exact: true });
  const data = useSyncExternalStore(subscribe, () => find()?.state.data);
  const observers = useSyncExternalStore(subscribe, () => find()?.getObserversCount() ?? 0);
  if (!enabled || !data || observers === 0 || data.staleIds.size > 0) return null;
  return data.rows.length >= SUBAGENT_LIST_PAGE_SIZE ? null : data;
}

/**
 * Direct subagents of one parent session (S9 preview, S10 roster). While the
 * parent's session stream is open its roster feeds the rows and no list request
 * runs; otherwise the paged list route serves a cold read. The population is
 * the parent's direct children, never the workspace.
 */
export function useSessionSubagents(
  workspaceId: string,
  sessionId: string,
  { enabled = true, allPages = true }: SessionSubagentsOptions = {}
): SessionSubagentsModel {
  const queryClient = useQueryClient();
  const scoped = enabled && workspaceId !== "" && sessionId !== "";
  const live = useLiveSubagentRoster(workspaceId, sessionId, scoped);
  const query = useInfiniteQuery({
    ...sessionSubagentsOptions(workspaceId, sessionId),
    enabled: scoped && live === null,
  });
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query;
  const walk = allPages && live === null;
  useEffect(() => {
    if (walk && hasNextPage && !isFetchingNextPage) void fetchNextPage().catch(() => undefined);
  }, [walk, hasNextPage, isFetchingNextPage, fetchNextPage]);

  return {
    subagents:
      live?.rows ??
      query.data?.pages.flatMap(page => page.subagents.map(subagentViewFromPayload)) ??
      [],
    stop: async subagent => {
      await cancelSubagent(workspaceId, subagent.id);
      // A live roster settles through the stream; a cold read re-reads the list.
      if (live === null) {
        await queryClient.invalidateQueries({
          queryKey: sessionKeys.subagents(workspaceId, sessionId),
        });
      }
    },
  };
}
