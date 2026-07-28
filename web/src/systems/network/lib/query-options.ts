import {
  infiniteQueryOptions,
  queryOptions,
  type QueryClient,
  type QueryKey,
} from "@tanstack/react-query";

import {
  NetworkApiError,
  getNetworkChannel,
  getNetworkDirectRoom,
  getNetworkStatus,
  getNetworkThread,
  listNetworkChannels,
  listNetworkDirectRoomMessages,
  listNetworkDirectRooms,
  listNetworkPeers,
  listNetworkSubscriptions,
  listNetworkThreadMessages,
  listNetworkThreads,
  type NetworkSubscriptionsListQuery,
} from "../adapters/network-api";
import {
  getNetworkCoordination,
  getNetworkUsage,
  type NetworkCoordinationRef,
} from "../adapters/network-coordination-api";
import type {
  NetworkChannelsQuery,
  NetworkConversationMessageFilters,
  NetworkCursorPageParam,
  NetworkDirectsListQuery,
  NetworkMessagePageParam,
  NetworkSurface,
  NetworkThreadsListQuery,
  NetworkUsageFilters,
} from "../types";
import {
  latestNetworkTailId,
  mergeNetworkTailResponse,
  type NetworkMessageData,
} from "./network-message-tail";
import {
  NETWORK_DEFAULT_LIST_LIMIT,
  NETWORK_DEFAULT_RECENTS_LIMIT,
  NETWORK_DEFAULT_TIMELINE_LIMIT,
} from "./query-constants";
import { networkKeys } from "./query-keys";

const STATUS_REFETCH_INTERVAL = 30_000;
const STATUS_STALE_TIME = 10_000;
const CHANNELS_REFETCH_INTERVAL = 30_000;
const LIST_REFETCH_INTERVAL = 15_000;
const LIST_STALE_TIME = 5_000;
const TAIL_REFRESH_INTERVAL = 5_000;
const DETAIL_RETRY_LIMIT = 2;

export {
  NETWORK_DEFAULT_LIST_LIMIT,
  NETWORK_DEFAULT_RECENTS_LIMIT,
  NETWORK_DEFAULT_TIMELINE_LIMIT,
} from "./query-constants";

function shouldRetryDetailQuery(failureCount: number, error: Error): boolean {
  if (error instanceof NetworkApiError && error.status >= 400 && error.status < 500) {
    return false;
  }
  return failureCount < DETAIL_RETRY_LIMIT;
}

type NetworkCatalogStableQuery = Omit<NetworkThreadsListQuery, "after"> & { limit: number };

function normalizeCatalogQuery(
  query: NetworkThreadsListQuery | NetworkDirectsListQuery,
  limit = NETWORK_DEFAULT_LIST_LIMIT
): NetworkCatalogStableQuery {
  const { after: _after, session_id: sessionId, query: search, sort, ...stable } = query;
  return {
    ...stable,
    ...(search?.trim() ? { query: search.trim() } : {}),
    ...(sessionId?.trim() ? { session_id: sessionId.trim() } : {}),
    limit: query.limit ?? limit,
    sort: sort?.trim() || "recent_activity",
  };
}

function normalizeMessageFilters(
  query: NetworkConversationMessageFilters,
  limit = NETWORK_DEFAULT_TIMELINE_LIMIT
): NetworkConversationMessageFilters & { limit: number } {
  const { kind, work_id: workId, ...stable } = query;
  return {
    ...stable,
    ...(kind?.trim() ? { kind: kind.trim() } : {}),
    ...(workId?.trim() ? { work_id: workId.trim() } : {}),
    limit: query.limit ?? limit,
  };
}

export function networkStatusOptions() {
  return queryOptions({
    queryKey: networkKeys.status(),
    queryFn: ({ signal }) => getNetworkStatus(signal),
    staleTime: STATUS_STALE_TIME,
    refetchInterval: STATUS_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
  });
}

export function networkCoordinationOptions(
  workspaceId: string,
  ref: NetworkCoordinationRef,
  enabled = true
) {
  return queryOptions({
    queryKey: networkKeys.coordination(workspaceId, ref),
    queryFn: ({ signal }) => getNetworkCoordination(workspaceId, ref, signal),
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function networkUsageOptions(
  workspaceId: string,
  filters: NetworkUsageFilters = {},
  enabled = true
) {
  return infiniteQueryOptions({
    queryKey: networkKeys.usage(workspaceId, filters),
    queryFn: ({ pageParam, signal }) =>
      getNetworkUsage(workspaceId, { ...filters, cursor: pageParam ?? undefined }, signal),
    initialPageParam: null as string | null,
    getNextPageParam: lastPage => lastPage.next_cursor || undefined,
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function networkChannelsOptions(
  workspaceId: string,
  query: NetworkChannelsQuery = { recent_limit: NETWORK_DEFAULT_RECENTS_LIMIT },
  enabled = true
) {
  const normalizedQuery = {
    ...query,
    recent_limit: query.recent_limit ?? NETWORK_DEFAULT_RECENTS_LIMIT,
  };
  return queryOptions({
    queryKey: networkKeys.channels(workspaceId, normalizedQuery),
    queryFn: ({ signal }) => listNetworkChannels(workspaceId, normalizedQuery, signal),
    staleTime: LIST_STALE_TIME,
    refetchInterval: CHANNELS_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && enabled,
  });
}

export function networkChannelDetailOptions(workspaceId: string, channel: string, enabled = true) {
  return queryOptions({
    queryKey: networkKeys.channelDetail(workspaceId, channel),
    queryFn: ({ signal }) => getNetworkChannel(workspaceId, channel, signal),
    staleTime: LIST_STALE_TIME,
    refetchInterval: LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && Boolean(channel) && enabled,
  });
}

export function networkSubscriptionsOptions(
  workspaceId: string,
  channel: string,
  query: NetworkSubscriptionsListQuery = {},
  enabled = true
) {
  return queryOptions({
    queryKey: networkKeys.subscriptions(workspaceId, channel, query),
    queryFn: ({ signal }) => listNetworkSubscriptions(workspaceId, channel, query, signal),
    staleTime: LIST_STALE_TIME,
    refetchInterval: LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && Boolean(channel) && enabled,
  });
}

export function networkThreadsOptions(
  workspaceId: string,
  channel: string,
  query: NetworkThreadsListQuery = {},
  enabled = true
) {
  const stableQuery = normalizeCatalogQuery(query);
  return infiniteQueryOptions({
    queryKey: networkKeys.threadsList(workspaceId, channel, stableQuery),
    queryFn: ({ pageParam, signal }) =>
      listNetworkThreads(
        workspaceId,
        channel,
        { ...stableQuery, ...(pageParam ? { after: pageParam } : {}) },
        signal
      ),
    initialPageParam: null as NetworkCursorPageParam,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: LIST_STALE_TIME,
    refetchInterval: query =>
      (query.state.data?.pages.length ?? 0) > 1 ? false : LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && Boolean(channel) && enabled,
  });
}

export function networkThreadDetailOptions(
  workspaceId: string,
  channel: string,
  threadId: string,
  enabled = true
) {
  return queryOptions({
    queryKey: networkKeys.threadDetail(workspaceId, channel, threadId),
    queryFn: ({ signal }) => getNetworkThread(workspaceId, channel, threadId, signal),
    retry: shouldRetryDetailQuery,
    staleTime: LIST_STALE_TIME,
    refetchInterval: LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && Boolean(channel) && Boolean(threadId) && enabled,
  });
}

export function networkThreadMessagesOptions(
  workspaceId: string,
  channel: string,
  threadId: string,
  query: NetworkConversationMessageFilters = {},
  enabled = true
) {
  const filters = normalizeMessageFilters(query);
  return infiniteQueryOptions({
    queryKey: networkKeys.threadMessages(workspaceId, channel, threadId, filters),
    queryFn: ({ pageParam, signal }) =>
      listNetworkThreadMessages(
        workspaceId,
        channel,
        threadId,
        { ...filters, ...pageParam },
        signal
      ),
    initialPageParam: null as NetworkMessagePageParam,
    getNextPageParam: lastPage =>
      lastPage.page.has_more && lastPage.page.next_cursor
        ? { before: lastPage.page.next_cursor }
        : undefined,
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
    enabled: Boolean(workspaceId) && Boolean(channel) && Boolean(threadId) && enabled,
  });
}

export function networkDirectsOptions(
  workspaceId: string,
  channel: string,
  query: NetworkDirectsListQuery = {},
  enabled = true
) {
  const stableQuery = normalizeCatalogQuery(query);
  return infiniteQueryOptions({
    queryKey: networkKeys.directsList(workspaceId, channel, stableQuery),
    queryFn: ({ pageParam, signal }) =>
      listNetworkDirectRooms(
        workspaceId,
        channel,
        { ...stableQuery, ...(pageParam ? { after: pageParam } : {}) },
        signal
      ),
    initialPageParam: null as NetworkCursorPageParam,
    getNextPageParam: lastPage => (lastPage.page.has_more ? lastPage.page.next_cursor : undefined),
    staleTime: LIST_STALE_TIME,
    refetchInterval: query =>
      (query.state.data?.pages.length ?? 0) > 1 ? false : LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && Boolean(channel) && enabled,
  });
}

export function networkDirectDetailOptions(
  workspaceId: string,
  channel: string,
  directId: string,
  enabled = true
) {
  return queryOptions({
    queryKey: networkKeys.directDetail(workspaceId, channel, directId),
    queryFn: ({ signal }) => getNetworkDirectRoom(workspaceId, channel, directId, signal),
    retry: shouldRetryDetailQuery,
    staleTime: LIST_STALE_TIME,
    refetchInterval: LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && Boolean(channel) && Boolean(directId) && enabled,
  });
}

export function networkDirectMessagesOptions(
  workspaceId: string,
  channel: string,
  directId: string,
  query: NetworkConversationMessageFilters = {},
  enabled = true
) {
  const filters = normalizeMessageFilters(query);
  return infiniteQueryOptions({
    queryKey: networkKeys.directMessages(workspaceId, channel, directId, filters),
    queryFn: ({ pageParam, signal }) =>
      listNetworkDirectRoomMessages(
        workspaceId,
        channel,
        directId,
        { ...filters, ...pageParam },
        signal
      ),
    initialPageParam: null as NetworkMessagePageParam,
    getNextPageParam: lastPage =>
      lastPage.page.has_more && lastPage.page.next_cursor
        ? { before: lastPage.page.next_cursor }
        : undefined,
    staleTime: Number.POSITIVE_INFINITY,
    refetchOnWindowFocus: false,
    enabled: Boolean(workspaceId) && Boolean(channel) && Boolean(directId) && enabled,
  });
}

export interface NetworkMessageTailQuery {
  workspaceId: string;
  channel: string;
  surface: NetworkSurface | null | undefined;
  containerId: string;
  filters: NetworkConversationMessageFilters;
  enabled: boolean;
}

export function networkMessageTailOptions(
  queryClient: QueryClient,
  { workspaceId, channel, surface, containerId, filters, enabled }: NetworkMessageTailQuery
) {
  const normalizedFilters = {
    ...filters,
    limit: filters.limit ?? NETWORK_DEFAULT_TIMELINE_LIMIT,
  };
  const canonicalKey: QueryKey =
    surface === "thread"
      ? networkThreadMessagesOptions(workspaceId, channel, containerId, normalizedFilters).queryKey
      : networkDirectMessagesOptions(workspaceId, channel, containerId, normalizedFilters).queryKey;
  const tailKey =
    surface === "thread"
      ? networkKeys.threadMessageTail(workspaceId, channel, containerId, normalizedFilters)
      : networkKeys.directMessageTail(workspaceId, channel, containerId, normalizedFilters);

  return queryOptions({
    queryKey: tailKey,
    queryFn: async ({ signal }) => {
      const current = queryClient.getQueryData<NetworkMessageData>(canonicalKey);
      const after = latestNetworkTailId(current);
      const query = { ...normalizedFilters, ...(after ? { after } : {}) };
      const response =
        surface === "thread"
          ? await listNetworkThreadMessages(workspaceId, channel, containerId, query, signal)
          : await listNetworkDirectRoomMessages(workspaceId, channel, containerId, query, signal);
      queryClient.setQueryData<NetworkMessageData>(canonicalKey, previous =>
        previous ? mergeNetworkTailResponse(previous, response, !after) : previous
      );
      return null;
    },
    enabled: enabled && Boolean(workspaceId) && Boolean(channel) && Boolean(containerId),
    refetchInterval: TAIL_REFRESH_INTERVAL,
    refetchOnWindowFocus: true,
    staleTime: 0,
  });
}

export function networkPeersOptions(workspaceId: string, channel?: string, enabled = true) {
  return queryOptions({
    queryKey: networkKeys.peers(workspaceId, channel),
    queryFn: ({ signal }) => listNetworkPeers(workspaceId, channel, signal),
    staleTime: LIST_STALE_TIME,
    refetchInterval: LIST_REFETCH_INTERVAL,
    refetchOnWindowFocus: true,
    enabled: Boolean(workspaceId) && enabled,
  });
}
