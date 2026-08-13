import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { useNetworkLiveDataEnabled } from "./use-network-live-data-enabled";

import { flattenNetworkDirects } from "../lib/infinite-data";
import { networkDirectDetailOptions, networkDirectsOptions } from "../lib/query-options";
import type {
  NetworkDirectRoomDetail,
  NetworkDirectRoomSummary,
  NetworkDirectsListQuery,
} from "../types";
import { useActiveWorkspace } from "@/systems/workspace";

export interface UseNetworkDirectsResult {
  directs: NetworkDirectRoomSummary[];
  total: number;
  hasMore: boolean;
  isLoading: boolean;
  isLoadingMore: boolean;
  loadMore: () => Promise<void>;
  error: Error | null;
}

export interface UseNetworkDirectsOptions {
  enabled?: boolean;
  workspaceId?: string | null;
  query?: Omit<NetworkDirectsListQuery, "after">;
  limit?: number;
  sessionId?: string;
}

export function useNetworkDirects(
  channel: string | null | undefined,
  options: UseNetworkDirectsOptions = {}
): UseNetworkDirectsResult {
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
    networkDirectsOptions(
      workspaceId,
      channel ?? "",
      {
        ...options.query,
        ...(options.limit ? { limit: options.limit } : {}),
        ...(options.sessionId ? { session_id: options.sessionId } : {}),
      },
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
    directs: flattenNetworkDirects(query.data),
    total: query.data?.pages[0]?.page.total ?? 0,
    hasMore: query.hasNextPage,
    isLoading: enabled && query.isLoading,
    isLoadingMore: query.isFetchingNextPage,
    loadMore,
    error: query.error ?? null,
  };
}

export interface UseNetworkDirectDetailResult {
  direct: NetworkDirectRoomDetail | null;
  isLoading: boolean;
  error: Error | null;
}

export function useNetworkDirectDetail(
  channel: string | null | undefined,
  directId: string | null | undefined,
  options: { enabled?: boolean; workspaceId?: string | null } = {}
): UseNetworkDirectDetailResult {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useNetworkLiveDataEnabled();
  const selectedWorkspaceId = options.workspaceId ?? runtimeWorkspaceId;
  const workspaceId = selectedWorkspaceId ?? "";
  const enabled =
    liveDataEnabled &&
    options.enabled !== false &&
    Boolean(channel) &&
    Boolean(directId) &&
    Boolean(selectedWorkspaceId);
  const query = useQuery(
    networkDirectDetailOptions(workspaceId, channel ?? "", directId ?? "", enabled)
  );
  return {
    direct: query.data ?? null,
    isLoading: enabled && query.isLoading,
    error: query.error ?? null,
  };
}
