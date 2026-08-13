import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { useNetworkLiveDataEnabled } from "./use-network-live-data-enabled";

import { flattenNetworkThreads } from "../lib/infinite-data";
import { networkThreadDetailOptions, networkThreadsOptions } from "../lib/query-options";
import type { NetworkThreadDetail, NetworkThreadsListQuery, NetworkThreadSummary } from "../types";
import { useActiveWorkspace } from "@/systems/workspace";

export interface UseNetworkThreadsResult {
  threads: NetworkThreadSummary[];
  total: number;
  hasMore: boolean;
  isLoading: boolean;
  isLoadingMore: boolean;
  loadMore: () => Promise<void>;
  error: Error | null;
}

export interface UseNetworkThreadsOptions {
  enabled?: boolean;
  workspaceId?: string | null;
  query?: Omit<NetworkThreadsListQuery, "after">;
  limit?: number;
}

export function useNetworkThreads(
  channel: string | null | undefined,
  options: UseNetworkThreadsOptions = {}
): UseNetworkThreadsResult {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useNetworkLiveDataEnabled();
  const selectedWorkspaceId = options.workspaceId ?? runtimeWorkspaceId;
  const workspaceId = selectedWorkspaceId ?? "";
  const enabled =
    liveDataEnabled &&
    options.enabled !== false &&
    Boolean(channel) &&
    Boolean(selectedWorkspaceId);
  const query = useInfiniteQuery(
    networkThreadsOptions(
      workspaceId,
      channel ?? "",
      { ...options.query, ...(options.limit ? { limit: options.limit } : {}) },
      enabled
    )
  );
  const loadMore = async () => {
    if (!query.hasNextPage || query.isFetchingNextPage) {
      return;
    }
    await query.fetchNextPage();
  };

  return {
    threads: flattenNetworkThreads(query.data),
    total: query.data?.pages[0]?.page.total ?? 0,
    hasMore: query.hasNextPage,
    isLoading: enabled && query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    loadMore,
    error: query.error ?? null,
  };
}

export interface UseNetworkThreadDetailResult {
  thread: NetworkThreadDetail | null;
  isLoading: boolean;
  error: Error | null;
}

export function useNetworkThreadDetail(
  channel: string | null | undefined,
  threadId: string | null | undefined,
  options: { enabled?: boolean; workspaceId?: string | null } = {}
): UseNetworkThreadDetailResult {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useNetworkLiveDataEnabled();
  const selectedWorkspaceId = options.workspaceId ?? runtimeWorkspaceId;
  const workspaceId = selectedWorkspaceId ?? "";
  const enabled =
    liveDataEnabled &&
    options.enabled !== false &&
    Boolean(channel) &&
    Boolean(threadId) &&
    Boolean(selectedWorkspaceId);
  const query = useQuery(
    networkThreadDetailOptions(workspaceId, channel ?? "", threadId ?? "", enabled)
  );
  return {
    thread: query.data ?? null,
    isLoading: enabled && query.isLoading,
    error: query.error ?? null,
  };
}
