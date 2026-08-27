import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  type AgentsFleetSearch,
  hasActiveAgentFleetFilters,
  projectAgentFleetRows,
  useAgentCatalog,
  useAgentCreateHost,
  useAgentFleetCallInstances,
} from "@/systems/agent";
import { useSessionCreateActions, useSessionCreateHasActiveWorkspace } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

const SEARCH_DEBOUNCE_MS = 200;

function useAgentsFleetPage(search: AgentsFleetSearch = {}) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const workspaceId = runtimeWorkspaceId ?? "";
  const navigate = useNavigate({ from: "/agents" });
  const { openDialog } = useAgentCreateHost();
  const { openForAgent } = useSessionCreateActions();
  const hasActiveWorkspace = useSessionCreateHasActiveWorkspace();
  const searchInputRef = useListingSearchShortcut(liveDataEnabled);
  const routeQuery = search.q ?? "";
  const updateSearch = (updater: (current: AgentsFleetSearch) => AgentsFleetSearch) => {
    void navigate({ search: current => updater(current), to: "/agents" });
  };
  const queryInput = useDebouncedInput({
    delayMs: SEARCH_DEBOUNCE_MS,
    externalValue: routeQuery,
    onCommit: nextQuery =>
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(nextQuery) })),
  });

  const agentsEnabled = liveDataEnabled && workspaceId !== "";
  const catalogQuery = useAgentCatalog(
    workspaceId,
    {
      q: search.q,
      category: search.category,
      status: search.status,
      limit: 50,
    },
    { enabled: agentsEnabled }
  );
  const agents = catalogQuery.agents.map(item => item.agent);
  const sessionsPartial = catalogQuery.isSuccess && !catalogQuery.sessionsAvailable;
  const view: ListingViewMode = search.view ?? "rows";

  const setFilters = (next: Pick<AgentsFleetSearch, "category" | "status">) => {
    updateSearch(current => ({ ...current, category: next.category, status: next.status }));
  };

  const setView = (nextView: ListingViewMode) => {
    updateSearch(current => ({
      ...current,
      view: nextView === "rows" ? undefined : nextView,
    }));
  };

  const clearFilters = () => {
    queryInput.reset("");
    updateSearch(current => ({
      view: current.view,
    }));
  };

  const openNewSession = (agentName: string) => {
    openForAgent(agentName);
  };

  const callInstances = useAgentFleetCallInstances(liveDataEnabled, agentsEnabled);
  const rows = projectAgentFleetRows({
    items: catalogQuery.agents,
    sessionsAvailable: catalogQuery.sessionsAvailable,
    callInstances,
  });

  const filtersActive = hasActiveAgentFleetFilters(search);
  const isLoading = catalogQuery.isLoading;
  const overallTotal = catalogQuery.facets?.total ?? 0;
  const isFirstRunEmpty =
    !isLoading && !catalogQuery.isError && overallTotal === 0 && !filtersActive;
  const isFilteredEmpty =
    !isLoading && !catalogQuery.isError && catalogQuery.total === 0 && filtersActive;
  const showFacets =
    !isLoading && !isFirstRunEmpty && !(catalogQuery.isError && agents.length === 0);
  // View toggle is independent of facets: available while loading/ready, hidden only when
  // there is no list surface (first-run empty or fatal empty error).
  const showViewToggle = !isFirstRunEmpty && !(catalogQuery.isError && agents.length === 0);

  return {
    workspaceId,
    catalogQuery,
    agents,
    fleetTotal: catalogQuery.total,
    rows,
    categoryOptions: catalogQuery.facets?.categories ?? [],
    search,
    draftQuery: queryInput.draftValue,
    searchInputRef,
    view,
    setDraftQuery: queryInput.setDraftValue,
    setFilters,
    setView,
    clearFilters,
    openCreate: openDialog,
    openNewSession,
    newSessionDisabled: !hasActiveWorkspace,
    isLoading,
    isFirstRunEmpty,
    isFilteredEmpty,
    sessionsPartial,
    hasMore: catalogQuery.hasNextPage,
    isLoadingMore: catalogQuery.isFetchingNextPage,
    loadMore: () => {
      void catalogQuery.fetchNextPage();
    },
    showFacets,
    showViewToggle,
    agentsError: catalogQuery.error,
    retryAgents: () => {
      void catalogQuery.refetch();
    },
  };
}

export { useAgentsFleetPage };
export type { AgentsFleetSearch };
