import { createStoreLogic } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useDeferredValue } from "react";

import { createFilter, type Filter as ReuiFilter } from "@compozy/ui";

import type { NetworkLastReadLookupKey } from "./use-last-read";
import { useNetworkListFiltersStore } from "./use-network-list-filters-context";
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

export interface NetworkListFiltersStoreInput {
  workspaceId: string;
  channel: string;
}

interface NetworkListFiltersStoreContext {
  workspaceId: string;
  channel: string;
  filters: NetworkChipFilter[];
  sort: NetworkListSort;
  searchQuery: string;
}

export const networkListFiltersLogic = createStoreLogic({
  context: (input: NetworkListFiltersStoreInput): NetworkListFiltersStoreContext =>
    initialFilterContext(input),
  on: {
    filtersChanged: (context, event: { filters: NetworkChipFilter[] }) => ({
      ...context,
      filters: event.filters.filter(filter => isKnownChipKey(filter.field)),
    }),
    searchEntered: (context, event: { searchQuery: string }) => ({
      ...context,
      searchQuery: event.searchQuery,
    }),
    sortSelected: (context, event: { sort: NetworkListSort }) => ({
      ...context,
      sort: event.sort,
    }),
    scopeChanged: (context, event: NetworkListFiltersStoreInput) => {
      if (context.workspaceId === event.workspaceId && context.channel === event.channel) {
        return;
      }
      return initialFilterContext(event);
    },
  },
});

export type NetworkListFiltersStore = ReturnType<typeof networkListFiltersLogic.createStore>;

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

export interface UseNetworkListFilterStateResult {
  filters: NetworkChipFilter[];
  sort: NetworkListSort;
  searchQuery: string;
  setFilters: (next: NetworkChipFilter[]) => void;
  setSort: (next: NetworkListSort) => void;
  setSearchQuery: (next: string) => void;
}

/**
 * Client-only list controls. Query, session, and read-state stay in their
 * respective owners and are composed by `useNetworkListFilters` below.
 */
export function useNetworkListFilterState(
  canFilterBySelf: boolean,
  scope: NetworkListFiltersStoreInput
): UseNetworkListFilterStateResult {
  const store = useNetworkListFiltersStore();
  const storedWorkspaceId = useSelector(store, snapshot => snapshot.context.workspaceId);
  const storedChannel = useSelector(store, snapshot => snapshot.context.channel);
  const storedFilters = useSelector(store, snapshot => snapshot.context.filters);
  const storedSort = useSelector(store, snapshot => snapshot.context.sort);
  const storedSearchQuery = useSelector(store, snapshot => snapshot.context.searchQuery);
  const scopeMatches = storedWorkspaceId === scope.workspaceId && storedChannel === scope.channel;
  const scopedFilters = scopeMatches ? storedFilters : [];
  const sort = scopeMatches ? storedSort : "recent_activity";
  const searchQuery = scopeMatches ? storedSearchQuery : "";
  const filters = canFilterBySelf
    ? scopedFilters
    : scopedFilters.filter(filter => filter.field !== "includes_me");
  const adoptScope = () => {
    if (!scopeMatches) store.trigger.scopeChanged(scope);
  };
  const setFilters = (next: NetworkChipFilter[]) => {
    adoptScope();
    store.trigger.filtersChanged({
      filters: next.filter(
        filter =>
          isKnownChipKey(filter.field) && (filter.field !== "includes_me" || canFilterBySelf)
      ),
    });
  };
  const setSort = (next: NetworkListSort) => {
    adoptScope();
    store.trigger.sortSelected({ sort: next });
  };
  const setSearchQuery = (next: string) => {
    adoptScope();
    store.trigger.searchEntered({ searchQuery: next });
  };
  return {
    filters,
    sort,
    searchQuery,
    setFilters,
    setSort,
    setSearchQuery,
  };
}

export function useNetworkListFilters({
  workspaceId,
  channel,
  enabled = true,
}: UseNetworkListFiltersArgs): UseNetworkListFiltersResult {
  const session = useActiveNetworkSession(channel, { workspaceId });
  const selfSessionId = session.session?.sessionId ?? null;
  const canFilterBySelf = Boolean(selfSessionId);
  const filterState = useNetworkListFilterState(canFilterBySelf, {
    workspaceId: workspaceId ?? "",
    channel,
  });
  const deferredSearch = useDeferredValue(filterState.searchQuery.trim());
  const selected = activeKeys(filterState.filters);
  const serverQuery = {
    ...(deferredSearch ? { query: deferredSearch } : {}),
    ...(selected.has("has_work") ? { has_work: true } : {}),
    ...(selected.has("includes_me") && selfSessionId ? { session_id: selfSessionId } : {}),
    sort: filterState.sort,
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
  const isFiltered = filterState.filters.length > 0 || filterState.searchQuery.trim().length > 0;

  return {
    ...filterState,
    canFilterBySelf,
    isFiltered,
    threadsQuery,
    directsQuery,
    filteredThreads: threadsQuery.threads,
    filteredDirects: directsQuery.directs,
    markLoadedRead,
    isMarkLoadedReadDisabled,
  };
}

function initialFilterContext(input: NetworkListFiltersStoreInput): NetworkListFiltersStoreContext {
  return {
    ...input,
    filters: [],
    sort: "recent_activity",
    searchQuery: "",
  };
}
