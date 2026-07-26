import { useDeferredValue, useState } from "react";

import { createFilter, type Filter as ReuiFilter } from "@agh/ui";

import type { NetworkLastReadLookupKey } from "./use-last-read";
import { useLastRead } from "./use-last-read";
import { useActiveNetworkSession } from "./use-active-session";
import { useNetworkDirects, type UseNetworkDirectsResult } from "./use-directs";
import { useNetworkThreads, type UseNetworkThreadsResult } from "./use-threads";

export type NetworkListSort = "recent_activity" | "created" | "alphabetical";
export type NetworkFilterKey = "has_work" | "includes_me";
export const NETWORK_FILTER_KEYS = [
  "has_work",
  "includes_me",
] as const satisfies ReadonlyArray<NetworkFilterKey>;

export type NetworkChipFilter = ReuiFilter<boolean>;

function isKnownChipKey(field: string): field is NetworkFilterKey {
  return (NETWORK_FILTER_KEYS as ReadonlyArray<string>).includes(field);
}

function activeKeys(filters: ReadonlyArray<NetworkChipFilter>): Set<NetworkFilterKey> {
  return new Set(
    filters
      .map(filter => filter.field)
      .filter((field): field is NetworkFilterKey => isKnownChipKey(field))
  );
}

export function createNetworkChipFilter(key: NetworkFilterKey): NetworkChipFilter {
  return createFilter<boolean>(key, "is", [true]);
}

export interface UseNetworkListFiltersArgs {
  workspaceId: string | null | undefined;
  channel: string;
  enabled?: boolean;
}

export interface UseNetworkListFiltersResult {
  filters: NetworkChipFilter[];
  sort: NetworkListSort;
  searchQuery: string;
  canFilterBySelf: boolean;
  isFiltered: boolean;
  setFilters: (next: NetworkChipFilter[]) => void;
  setSort: (next: NetworkListSort) => void;
  setSearchQuery: (next: string) => void;
  threadsQuery: UseNetworkThreadsResult;
  directsQuery: UseNetworkDirectsResult;
  filteredThreads: UseNetworkThreadsResult["threads"];
  filteredDirects: UseNetworkDirectsResult["directs"];
  markLoadedRead: () => void;
  isMarkLoadedReadDisabled: boolean;
}

export function useNetworkListFilters({
  workspaceId,
  channel,
  enabled = true,
}: UseNetworkListFiltersArgs): UseNetworkListFiltersResult {
  const [filters, setFilterState] = useState<NetworkChipFilter[]>([]);
  const [sort, setSort] = useState<NetworkListSort>("recent_activity");
  const [searchQuery, setSearchQuery] = useState("");
  const deferredSearch = useDeferredValue(searchQuery.trim());
  const session = useActiveNetworkSession(channel, { workspaceId });
  const selfSessionId = session.session?.sessionId ?? null;
  const canFilterBySelf = Boolean(selfSessionId);
  const effectiveFilters = canFilterBySelf
    ? filters
    : filters.filter(filter => filter.field !== "includes_me");
  const selected = activeKeys(effectiveFilters);

  const setFilters = (next: NetworkChipFilter[]) => {
    setFilterState(
      next.filter(
        filter =>
          isKnownChipKey(filter.field) && (filter.field !== "includes_me" || canFilterBySelf)
      )
    );
  };
  const serverQuery = {
    ...(deferredSearch ? { query: deferredSearch } : {}),
    ...(selected.has("has_work") ? { has_work: true } : {}),
    ...(selected.has("includes_me") && selfSessionId ? { session_id: selfSessionId } : {}),
    sort,
  };
  const queryEnabled = enabled && Boolean(workspaceId) && Boolean(channel);
  const threadsQuery = useNetworkThreads(channel, {
    workspaceId,
    enabled: queryEnabled,
    query: serverQuery,
  });
  const directsQuery = useNetworkDirects(channel, {
    workspaceId,
    enabled: queryEnabled,
    query: serverQuery,
  });
  const lastRead = useLastRead({ workspaceId });

  const isUnread = (key: NetworkLastReadLookupKey, lastActivityAt?: string | null) => {
    if (!lastActivityAt) return false;
    const readAt = lastRead.lastReadAt(key);
    return !readAt || new Date(lastActivityAt).getTime() > new Date(readAt).getTime();
  };
  const markLoadedRead = () => {
    for (const thread of threadsQuery.threads) {
      lastRead.markRead(
        { channel, surface: "thread", containerId: thread.thread_id },
        thread.last_activity_at
      );
    }
    for (const direct of directsQuery.directs) {
      lastRead.markRead(
        { channel, surface: "direct", containerId: direct.direct_id },
        direct.last_activity_at
      );
    }
  };
  const hasUnreadThread = threadsQuery.threads.some(thread =>
    isUnread({ channel, surface: "thread", containerId: thread.thread_id }, thread.last_activity_at)
  );
  const hasUnreadDirect = directsQuery.directs.some(direct =>
    isUnread({ channel, surface: "direct", containerId: direct.direct_id }, direct.last_activity_at)
  );
  const isMarkLoadedReadDisabled = !hasUnreadThread && !hasUnreadDirect;
  const isFiltered = effectiveFilters.length > 0 || searchQuery.trim().length > 0;

  return {
    filters: effectiveFilters,
    sort,
    searchQuery,
    canFilterBySelf,
    isFiltered,
    setFilters,
    setSort,
    setSearchQuery,
    threadsQuery,
    directsQuery,
    filteredThreads: threadsQuery.threads,
    filteredDirects: directsQuery.directs,
    markLoadedRead,
    isMarkLoadedReadDisabled,
  };
}
