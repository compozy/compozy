import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@agh/ui";

import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";
import {
  bridgeListFilterForScope,
  parseBridgePlatformFilter,
  parseBridgeScopeFilter,
  parseBridgeStatusFilter,
  useBridgeHealthStream,
  useBridgeProviders,
  useBridges,
  type BridgePlatformFilter,
  type BridgeScopeFilter,
  type BridgeStatusFilter,
} from "@/systems/bridges";
import { useActiveWorkspace } from "@/systems/workspace";
import { useBridgeCreateFlow } from "@/hooks/routes/use-bridge-create-flow";

export interface BridgesRouteSearch {
  q?: string;
  view?: ListingViewMode;
  scope?: BridgeScopeFilter;
  platform?: BridgePlatformFilter;
  status?: BridgeStatusFilter;
}

function useBridgesPage(search: BridgesRouteSearch = {}) {
  const navigate = useNavigate({ from: "/bridges" });

  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();

  const searchQuery = search.q ?? "";
  const view: ListingViewMode = search.view ?? "rows";
  const scopeFilter: BridgeScopeFilter = search.scope ?? "all";
  const platformFilter = search.platform ?? null;
  const statusFilter = search.status ?? null;

  const bridgeListFilters = {
    ...bridgeListFilterForScope(scopeFilter, activeWorkspaceId),
    q: searchQuery,
    platform: platformFilter ?? undefined,
    status: statusFilter ?? undefined,
    sort: "name" as const,
    limit: 50,
  };
  const bridgeListEnabled = scopeFilter !== "workspace" || Boolean(activeWorkspaceId);

  const bridgesQuery = useBridges(bridgeListFilters, { enabled: bridgeListEnabled });
  useBridgeHealthStream({
    bridgeIds: bridgesQuery.bridges.map(bridge => bridge.id),
    enabled: bridgeListEnabled,
    filters: bridgeListFilters,
  });
  const providersQuery = useBridgeProviders();

  const bridges = bridgesQuery.bridges;
  const bridgeHealth = bridgesQuery.bridgeHealth;
  const providers = providersQuery.data ?? [];
  const createFlow = useBridgeCreateFlow({
    activeWorkspaceId,
    activeWorkspaceName: activeWorkspace?.name,
    providers,
  });

  const platforms = Object.keys(bridgesQuery.facets?.platforms ?? {});
  const statuses: BridgeStatusFilter[] = [];
  for (const [status, count] of Object.entries(bridgesQuery.facets?.statuses ?? {})) {
    if (count > 0 || status === statusFilter) statuses.push(status as BridgeStatusFilter);
  }
  const totalBridgeCount = bridgesQuery.total;
  const hasActiveFilters =
    searchQuery.trim() !== "" ||
    scopeFilter !== "all" ||
    platformFilter !== null ||
    statusFilter !== null;

  const isInitialLoading =
    (bridgesQuery.isLoading && !bridgesQuery.data) ||
    (providersQuery.isLoading && !providersQuery.data);
  const fatalError =
    (!bridgesQuery.data && bridgesQuery.error) || (!providersQuery.data && providersQuery.error);
  const backgroundError =
    bridgesQuery.error && bridgesQuery.data
      ? bridgesQuery.error
      : providersQuery.error && providersQuery.data
        ? providersQuery.error
        : null;

  const updateSearch = (updater: (current: BridgesRouteSearch) => BridgesRouteSearch) => {
    void navigate({
      search: current => updater((current as BridgesRouteSearch | undefined) ?? {}),
      to: "/bridges",
    });
  };

  const setSearchQuery = (nextQuery: string) => {
    updateSearch(current => ({
      ...current,
      q: normalizeListingSearchValue(nextQuery),
    }));
  };

  const setView = (nextView: ListingViewMode) => {
    updateSearch(current => ({
      ...current,
      view: nextView === "rows" ? undefined : nextView,
    }));
  };

  const setScopeFilter = (next: BridgeScopeFilter) => {
    updateSearch(current => ({
      ...current,
      scope: next === "all" ? undefined : next,
    }));
  };

  const setPlatformFilter = (next: BridgePlatformFilter | null) => {
    updateSearch(current => ({
      ...current,
      platform: next ?? undefined,
    }));
  };

  const setStatusFilter = (next: BridgeStatusFilter | null) => {
    updateSearch(current => ({
      ...current,
      status: next ?? undefined,
    }));
  };

  const clearFilters = () => {
    updateSearch(current => ({
      ...current,
      platform: undefined,
      q: undefined,
      scope: undefined,
      status: undefined,
    }));
  };

  const handleRefresh = () => {
    void bridgesQuery.refetch();
    void providersQuery.refetch();
  };

  return {
    activeWorkspaceId,
    backgroundError,
    bridgeHealth,
    bridges,
    canCreateBridge: createFlow.canCreateBridge,
    clearFilters,
    createDialogProps: createFlow.createDialogProps,
    fatalError,
    hasActiveFilters,
    hasNextPage: bridgesQuery.hasNextPage,
    handleRefresh,
    isInitialLoading,
    isFetchingNextPage: bridgesQuery.isFetchingNextPage,
    loadMore: bridgesQuery.fetchNextPage,
    openCreateDialog: createFlow.openCreateDialog,
    platformFilter,
    platforms,
    providers,
    scopeFilter,
    searchQuery,
    setPlatformFilter,
    setScopeFilter,
    setSearchQuery,
    setStatusFilter,
    setView,
    statusFilter,
    statuses,
    totalBridgeCount,
    view,
  };
}

export {
  parseBridgePlatformFilter,
  parseBridgeScopeFilter,
  parseBridgeStatusFilter,
  useBridgesPage,
};

export function validateBridgesSearch(search: Record<string, unknown>): BridgesRouteSearch {
  return {
    platform: parseBridgePlatformFilter(search.platform),
    q: normalizeListingSearchValue(search.q),
    scope: parseBridgeScopeFilter(search.scope),
    status: parseBridgeStatusFilter(search.status),
    view: parseListingView(search.view),
  };
}
