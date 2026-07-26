import { useEffect, useRef, useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@agh/ui";

import {
  hasActiveAgentFleetFilters,
  projectAgentFleetRows,
  useAgentCatalog,
  useAgentCreateHost,
  type AgentsFleetSearch,
} from "@/systems/agent";
import { useSessionCreate } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";
import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";

const SEARCH_DEBOUNCE_MS = 200;

function useAgentsFleetPage(search: AgentsFleetSearch = {}) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const navigate = useNavigate({ from: "/agents" });
  const { openDialog } = useAgentCreateHost();
  const sessionCreate = useSessionCreate();
  const searchInputRef = useListingSearchShortcut(true);
  const routeQuery = search.q ?? "";
  const [draftQueryState, setDraftQueryState] = useState({ routeQuery, value: routeQuery });
  const draftQuery = draftQueryState.routeQuery === routeQuery ? draftQueryState.value : routeQuery;
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const agentsEnabled = workspaceId !== "";
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

  useEffect(() => {
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, [routeQuery]);

  const updateSearch = (updater: (current: AgentsFleetSearch) => AgentsFleetSearch) => {
    void navigate({ search: current => updater(current), to: "/agents" });
  };

  const setDraftQueryAndDebounce = (nextQuery: string) => {
    setDraftQueryState({ routeQuery, value: nextQuery });
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      debounceRef.current = null;
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(nextQuery) }));
    }, SEARCH_DEBOUNCE_MS);
  };

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
    if (debounceRef.current) clearTimeout(debounceRef.current);
    setDraftQueryState({ routeQuery, value: "" });
    updateSearch(current => ({
      view: current.view,
    }));
  };

  const openNewSession = (agentName: string) => {
    sessionCreate.openForAgent(agentName);
  };

  const rows = projectAgentFleetRows({
    items: catalogQuery.agents,
    sessionsAvailable: catalogQuery.sessionsAvailable,
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
    draftQuery,
    searchInputRef,
    view,
    setDraftQuery: setDraftQueryAndDebounce,
    setFilters,
    setView,
    clearFilters,
    openCreate: openDialog,
    openNewSession,
    newSessionDisabled: !sessionCreate.hasActiveWorkspace,
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
