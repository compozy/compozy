import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  type AgentFleetRowModel,
  type AgentsFleetSearch,
  hasActiveAgentFleetFilters,
  projectAgentFleetRows,
  useAgentAggregate,
  useAgentCatalog,
  useAgentCreateHost,
} from "@/systems/agent";
import { useProfileReadScope } from "@/systems/profiles";
import { useSessionCreateActions, useSessionCreateHasActiveWorkspace } from "@/systems/session";
import {
  focusedWorkspaceSelector,
  ownerDataScope,
  useActiveWorkspace,
  useProjectFilter,
  useWindowDataWorkspace,
  workspaceBadgeIdentity,
} from "@/systems/workspace";
import { useOpenTarget } from "../../hooks/use-open-target";

const SEARCH_DEBOUNCE_MS = 200;

function useAgentsFleetPage(search: AgentsFleetSearch = {}) {
  const { pending, workspaces } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const openItem = useOpenTarget();
  // The window's scope: a project's agents, User's (`@user`), or under All
  // projects every project's agents plus User's, one row per (project, name).
  const dataScope = useWindowDataWorkspace();
  const allProjects = dataScope?.kind === "all";
  const workspaceId = focusedWorkspaceSelector(dataScope);
  const projectFilter = useProjectFilter(dataScope, workspaces);
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
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

  const agentsEnabled = liveDataEnabled && !pending && dataScope !== null;
  const filters = { q: search.q, category: search.category, status: search.status, limit: 50 };
  const projectCatalog = useAgentCatalog(workspaceId, filters, {
    enabled: agentsEnabled && !allProjects,
  });
  const aggregateCatalog = useAgentAggregate(projectFilter.values, filters, {
    enabled: agentsEnabled && allProjects,
  });
  const catalogQuery = allProjects ? aggregateCatalog : projectCatalog;
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
    projectFilter.setValues([]);
    updateSearch(current => ({
      view: current.view,
    }));
  };

  const openNewSession = (agentName: string) => {
    openForAgent(agentName);
  };

  const workspaceNames = new Map(workspaces.map(workspace => [workspace.id, workspace.name]));
  const rows = projectAgentFleetRows({
    items: catalogQuery.agents,
    sessionsAvailable: catalogQuery.sessionsAvailable,
    ...(allProjects
      ? { projectOf: agent => workspaceBadgeIdentity(agent.workspace_id, workspaceNames) }
      : {}),
  });
  // All projects: an agent opens in place, its window bound to the agent's project.
  const openAgent = (row: AgentFleetRowModel) =>
    openItem({
      app: "agents",
      binding: ownerDataScope(row.agent.workspace_id),
      route: { pathname: `/agents/${encodeURIComponent(row.agent.name)}`, search: {} },
    });

  const { isLoading, isFirstRunEmpty, isFilteredEmpty, showFacets, showViewToggle } =
    projectFleetVisibility(catalogQuery, pending, search, projectFilter.values.length > 0);

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
    // The rows of the previous search have a cursor of their own; the next page is the new search's.
    hasMore: catalogQuery.hasNextPage && !catalogQuery.isPlaceholderData,
    isLoadingMore: catalogQuery.isFetchingNextPage,
    loadMore: () => {
      void catalogQuery.fetchNextPage();
    },
    openAgent: allProjects ? openAgent : undefined,
    projectField: projectFilter.field,
    projectFilter: projectFilter.values,
    setProjectFilter: projectFilter.setValues,
    /** All profiles reads definitions for the acting profile only (configuration resolves per profile). */
    definitionsFor: profile.aggregate ? profile.destination : null,
    showFacets,
    showViewToggle,
    agentsError: catalogQuery.error,
    retryAgents: () => {
      void catalogQuery.refetch();
    },
  };
}

/** What the fleet's visibility reads of a catalog query; both catalog hooks satisfy it. */
interface FleetCatalogState {
  agents: readonly unknown[];
  facets?: { total: number };
  isError: boolean;
  isPending: boolean;
  isPlaceholderData: boolean;
  total: number;
}

function projectFleetVisibility(
  catalogQuery: FleetCatalogState,
  pending: boolean,
  search: AgentsFleetSearch,
  projectFilterActive: boolean
) {
  const filtersActive = hasActiveAgentFleetFilters(search) || projectFilterActive;
  // No answer yet, however it is pending: still fetching, or idle until the window may read. A
  // placeholder keeps the previous search's rows on screen, but with none it is no answer.
  const isLoading =
    pending ||
    catalogQuery.isPending ||
    (catalogQuery.isPlaceholderData && catalogQuery.agents.length === 0);
  const overallTotal = catalogQuery.facets?.total ?? 0;
  const isFirstRunEmpty =
    !isLoading && !catalogQuery.isError && overallTotal === 0 && !filtersActive;
  const isFilteredEmpty =
    !isLoading && !catalogQuery.isError && catalogQuery.total === 0 && filtersActive;
  const showFacets =
    !isLoading && !isFirstRunEmpty && !(catalogQuery.isError && catalogQuery.agents.length === 0);
  // View toggle is independent of facets: available while loading/ready, hidden only when
  // there is no list surface (first-run empty or fatal empty error).
  const showViewToggle =
    !isFirstRunEmpty && !(catalogQuery.isError && catalogQuery.agents.length === 0);

  return { isLoading, isFirstRunEmpty, isFilteredEmpty, showFacets, showViewToggle };
}

export { projectFleetVisibility, useAgentsFleetPage };
export type { AgentsFleetSearch };
