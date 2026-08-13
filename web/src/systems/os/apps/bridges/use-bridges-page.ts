import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";

import type { BridgesRouteSearch } from "@/systems/bridges";

import { useBridgeCreateFlow } from "@/hooks/routes/use-bridge-create-flow";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  bridgeListFilterForScope,
  type BridgePlatformFilter,
  type BridgeScopeFilter,
  type BridgeStatusFilter,
  useBridgeHealthStream,
  useBridgeProviders,
  useBridges,
} from "@/systems/bridges";
import { useActiveWorkspace } from "@/systems/workspace";

function useBridgesPage(search: BridgesRouteSearch = {}) {
  const navigate = useNavigate({ from: "/bridges" });

  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();

  const routeSearchQuery = search.q ?? "";
  const view: ListingViewMode = search.view ?? "rows";
  const scopeFilter: BridgeScopeFilter = search.scope ?? "all";
  const platformFilter = search.platform ?? null;
  const statusFilter = search.status ?? null;

  const updateSearch = (updater: (current: BridgesRouteSearch) => BridgesRouteSearch) => {
    void navigate({
      search: current => updater((current as BridgesRouteSearch | undefined) ?? {}),
      to: "/bridges",
    });
  };
  const searchInput = useDebouncedInput({
    externalValue: routeSearchQuery,
    onCommit: nextQuery =>
      updateSearch(current => ({
        ...current,
        q: normalizeListingSearchValue(nextQuery),
      })),
  });
  const searchQuery = searchInput.draftValue;
  const bridgeListFilters = {
    ...bridgeListFilterForScope(scopeFilter, activeWorkspaceId),
    q: normalizeListingSearchValue(searchInput.committedValue) ?? "",
    platform: platformFilter ?? undefined,
    status: statusFilter ?? undefined,
    sort: "name" as const,
    limit: 50,
  };
  const bridgeListEnabled =
    liveDataEnabled && (scopeFilter !== "workspace" || Boolean(activeWorkspaceId));

  const bridgesQuery = useBridges(bridgeListFilters, { enabled: bridgeListEnabled });
  useBridgeHealthStream({
    bridgeIds: bridgesQuery.bridges.map(bridge => bridge.id),
    enabled: bridgeListEnabled,
    filters: bridgeListFilters,
  });
  const providersQuery = useBridgeProviders({ enabled: liveDataEnabled });

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
    searchInput.reset("");
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
    setSearchQuery: searchInput.setDraftValue,
    setStatusFilter,
    setView,
    statusFilter,
    statuses,
    totalBridgeCount,
    view,
  };
}

export { useBridgesPage };
export type { BridgesRouteSearch } from "@/systems/bridges";
