import { useNavigate } from "@tanstack/react-router";

import type { AgentPayload } from "@/systems/agent";
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
  useAgents,
  useAgentCreateHost,
} from "@/systems/agent";
import {
  agentShadowLayers,
  formatAgentFleetAriaLabel,
  formatAgentFleetCardCategory,
  formatAgentFleetMeta,
  formatAgentLayer,
  formatAgentOriginLabel,
} from "@/systems/agent/lib/agent-fleet-projection";
import type { AgentFleetRowModel } from "@/systems/agent/lib/agent-fleet-projection";
import { useSessionCreateActions, useSessionCreateHasActiveWorkspace } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

const SEARCH_DEBOUNCE_MS = 200;

function projectGlobalRows(agents: readonly AgentPayload[]): AgentFleetRowModel[] {
  return agents.map(agent => ({
    agent,
    signals: null,
    meta: formatAgentFleetMeta(agent),
    cardCategory: formatAgentFleetCardCategory(agent),
    cardOrigin: formatAgentOriginLabel(agent.origin),
    layer: formatAgentLayer(agent),
    shadowLayers: agentShadowLayers(agent),
    ariaLabel: formatAgentFleetAriaLabel(agent, null, false),
    hasDiagnostics: Array.isArray(agent.diagnostics) && agent.diagnostics.length > 0,
    sessionsAvailable: false,
  }));
}

function matchesGlobalAgent(agent: AgentPayload, search: AgentsFleetSearch): boolean {
  if (search.q) {
    const q = search.q.trim().toLowerCase();
    if (q && !agent.name.toLowerCase().includes(q)) return false;
  }
  if (search.category) {
    const category = search.category.trim();
    if (category) {
      const paths = agent.category_path ?? [];
      const formatted = paths.join(" / ");
      const raw = paths.join("/");
      if (!paths.includes(category) && formatted !== category && raw !== category) return false;
    }
  }
  return true;
}

function useAgentsFleetPage(search: AgentsFleetSearch = {}) {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const workspaceId = runtimeWorkspaceId ?? "";
  const isGlobal = workspaceId === "";
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

  const agentsEnabled = liveDataEnabled && !isGlobal;
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

  const globalQuery = useAgents(null, { enabled: liveDataEnabled && isGlobal });

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

  if (isGlobal) {
    const allGlobalAgents = globalQuery.data ?? [];
    const filteredAgents = allGlobalAgents.filter(agent => matchesGlobalAgent(agent, search));
    const filtersActive = Boolean(search.q?.trim() || search.category?.trim());
    const isLoading = globalQuery.isLoading;
    const isError = Boolean(globalQuery.isError);
    const overallTotal = allGlobalAgents.length;
    const isFirstRunEmpty =
      !isLoading && !globalQuery.isError && overallTotal === 0 && !filtersActive;
    const isFilteredEmpty =
      !isLoading && !globalQuery.isError && filteredAgents.length === 0 && filtersActive;
    const rows = projectGlobalRows(filteredAgents);
    const categoryOptions = [
      ...new Set(
        allGlobalAgents.flatMap(agent =>
          (agent.category_path ?? []).flatMap(value => {
            const trimmed = value.trim();
            return trimmed ? [trimmed] : [];
          })
        )
      ),
    ];
    const showFacets = !isLoading && !isFirstRunEmpty && !(isError && filteredAgents.length === 0);
    const showViewToggle = !isFirstRunEmpty && !(isError && filteredAgents.length === 0);

    return {
      workspaceId,
      catalogQuery: globalQuery as unknown as typeof catalogQuery,
      agents: filteredAgents,
      fleetTotal: filteredAgents.length,
      rows,
      categoryOptions,
      search,
      draftQuery: queryInput.draftValue,
      searchInputRef,
      view: (search.view ?? "rows") as ListingViewMode,
      setDraftQuery: queryInput.setDraftValue,
      setFilters,
      setView,
      clearFilters,
      openCreate: openDialog,
      openNewSession,
      newSessionDisabled: true,
      isLoading,
      isFirstRunEmpty,
      isFilteredEmpty,
      sessionsPartial: false,
      hasMore: false,
      isLoadingMore: false,
      loadMore: () => {},
      showFacets,
      showViewToggle,
      agentsError: globalQuery.error as unknown as typeof catalogQuery.error,
      retryAgents: () => {
        void globalQuery.refetch();
      },
    };
  }

  const agents = catalogQuery.agents.map(item => item.agent);
  const sessionsPartial = catalogQuery.isSuccess && !catalogQuery.sessionsAvailable;
  const view: ListingViewMode = search.view ?? "rows";

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
