import {
  hashKey,
  notifyManager,
  useInfiniteQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { useCallback, useEffect, useSyncExternalStore } from "react";

import { cancelSubagent } from "../adapters/subagent-api";
import type { SubagentView } from "../components/subagents/types";
import { sessionDetailOptions } from "../lib/query-options";
import { sessionKeys } from "../lib/query-keys";
import { subagentViewFromPayload } from "../lib/subagent-payload";
import { SUBAGENT_LIST_PAGE_SIZE, sessionSubagentsOptions } from "../lib/subagent-query-options";
import type { SubagentRoster } from "../lib/subagent-roster";

function subscribeIdle(): () => void {
  return () => undefined;
}

export interface SessionSubagentsModel {
  subagents: readonly SubagentView[];
  /**
   * The rows are the whole answer, even when empty: the stream roster, or a list
   * read with every page in.
   */
  complete: boolean;
  /** A complete answer confirmed by the live stream or a finished read after mounting. */
  confirmed: boolean;
  /** The list read failed and no stream roster stands in. */
  failed: boolean;
  /** Re-reads the list after a failure. */
  retry: () => void;
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
  /**
   * Fall back to the paged list route while the stream roster is not live.
   * False reads only the stream roster: a host that is always mounted reads
   * the list once the session reports subagents or the operator asks.
   */
  coldRead?: boolean;
  /** Confirm cached absence with a new read for this observer. */
  confirmOnMount?: boolean;
}

/**
 * The parent's stream-fed roster, only while it is live: the parent's thread is
 * mounted and observing it (that thread owns the session stream writing it), a
 * snapshot arrived, and no row awaits reconnect confirmation. It is read
 * without adding an observer, so the observer count is exactly the threads
 * holding the stream. The snapshot holds the newest 200 rows, so a full
 * snapshot defers to the list route.
 */
export function useLiveSubagentRoster(
  workspaceId: string,
  sessionId: string,
  enabled: boolean
): SubagentRoster | null {
  const cache = useQueryClient().getQueryCache();
  const queryHash = hashKey(sessionKeys.subagentRoster(workspaceId, sessionId));
  // Other sessions and background queries must not wake every roster reader.
  const subscribe = useCallback(
    (notify: () => void) => {
      const scheduledNotify = notifyManager.batchCalls(notify);
      return cache.subscribe(event => {
        if (event.query.queryHash === queryHash) scheduledNotify();
      });
    },
    [cache, queryHash]
  );
  const find = useCallback(() => cache.get<SubagentRoster>(queryHash), [cache, queryHash]);
  const readData = useCallback(() => find()?.state.data, [find]);
  const readObservers = useCallback(() => find()?.getObserversCount() ?? 0, [find]);
  const data = useSyncExternalStore(enabled ? subscribe : subscribeIdle, readData);
  const observers = useSyncExternalStore(enabled ? subscribe : subscribeIdle, readObservers);
  if (!enabled || !data || observers === 0 || data.snapshotPending || data.staleIds.size > 0)
    return null;
  return data.rows.length >= SUBAGENT_LIST_PAGE_SIZE ? null : data;
}

/**
 * Direct subagents of one parent session (S9 preview, S10 roster). While the
 * parent's session stream is open its roster feeds the rows and no list request
 * runs; otherwise the paged list route serves a cold read. The population is
 * the parent's direct children, never the workspace. Failed page walks wait for
 * explicit retry; cancellation refreshes the parent owner before dispatch.
 */
export function useSessionSubagents(
  workspaceId: string,
  sessionId: string,
  {
    enabled = true,
    allPages = true,
    coldRead = true,
    confirmOnMount = false,
  }: SessionSubagentsOptions = {}
): SessionSubagentsModel {
  const queryClient = useQueryClient();
  const scoped = enabled && workspaceId !== "" && sessionId !== "";
  const live = useLiveSubagentRoster(workspaceId, sessionId, scoped);
  const query = usePagedSubagents(workspaceId, sessionId, scoped && coldRead && live === null, {
    allPages,
    confirmOnMount,
  });
  const complete = query.isSuccess && !query.hasNextPage;

  return {
    subagents:
      live?.rows ??
      query.data?.pages.flatMap(page => page.subagents.map(subagentViewFromPayload)) ??
      [],
    complete: live !== null || complete,
    confirmed: live !== null || (complete && query.isFetchedAfterMount && !query.isFetching),
    failed: live === null && query.isError,
    retry: () => void query.refetch(),
    stop: subagent => stopSubagent(queryClient, workspaceId, sessionId, live !== null, subagent),
  };
}

/** Own the cold list read and its page walk separately from live roster selection. */
function usePagedSubagents(
  workspaceId: string,
  sessionId: string,
  enabled: boolean,
  { allPages, confirmOnMount }: SessionSubagentsOptions
) {
  const query = useInfiniteQuery({
    ...sessionSubagentsOptions(workspaceId, sessionId),
    enabled,
    ...(confirmOnMount ? { refetchOnMount: "always" as const } : {}),
  });
  const { hasNextPage, isFetching, isFetchingNextPage, fetchNextPage, isError } = query;
  useEffect(() => {
    if (enabled && allPages && hasNextPage && !isFetching && !isFetchingNextPage && !isError)
      void fetchNextPage().catch(() => undefined);
  }, [enabled, allPages, hasNextPage, isFetching, isFetchingNextPage, fetchNextPage, isError]);
  return query;
}

/** Resolve the current parent owner before cancellation, then refresh cold readers. */
async function stopSubagent(
  queryClient: QueryClient,
  workspaceId: string,
  sessionId: string,
  live: boolean,
  subagent: SubagentView
): Promise<void> {
  const parent = await queryClient.fetchQuery({
    ...sessionDetailOptions(workspaceId, sessionId),
    staleTime: 0,
  });
  await cancelSubagent(workspaceId, subagent.id, parent.profile_name);
  if (!live) {
    await queryClient.invalidateQueries({
      queryKey: sessionKeys.subagents(workspaceId, sessionId),
    });
  }
}
