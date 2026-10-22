import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import type { LoopsRouteSearch } from "@/systems/loops";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import {
  loopCatalogBatchEntries,
  type LoopCatalogEntry,
  type LoopCatalogFilter,
  useLoops,
  useLoopsAggregate,
} from "@/systems/loops";
import { useProfileReadScope } from "@/systems/profiles";
import {
  breadthQuery,
  itemBindingSearch,
  loopBreadthTarget,
  useActiveWorkspace,
  useProjectFilter,
  workspaceBadgeIdentity,
} from "@/systems/workspace";
import { useLoopWindowScope } from "./hooks/use-loop-window-scope";
import {
  loopCatalogFilterFromSearch,
  loopsCatalogAggregate,
  loopsCatalogQueryState,
  loopsCategoryOptions,
  loopsHasActiveFilters,
  loopsServerFilters,
  loopsWorkspaceLabel,
} from "./loops-catalog-model";

/**
 * View-model for the Loops catalog route: URL state, data, bindings, and Run
 * launch. A project's (or User's) catalog pages through its workspace route;
 * under All projects the catalog is the bounded definitions batch with a
 * project badge per row, per-project inline errors, and the Project filter.
 */
function useLoopsCatalog(search: LoopsRouteSearch = {}) {
  const { scope, workspaceId, search: windowSearch } = useLoopWindowScope();
  const { registeredWorkspaces, workspaces } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const navigate = useNavigate({ from: "/loops" });
  const allProjects = scope?.kind === "all";
  const projectFilter = useProjectFilter(scope, workspaces);

  const searchQuery = search.q ?? "";
  const view: ListingViewMode = search.view ?? "rows";
  const filter = loopCatalogFilterFromSearch(search);
  const serverFilters = loopsServerFilters(filter, searchQuery);
  const loopsQuery = useLoops(workspaceId, { ...serverFilters, limit: 50 }, workspaceId !== "");
  const target = loopBreadthTarget(breadthQuery({ kind: "all" }, projectFilter.values));
  const batchQuery = useLoopsAggregate(
    target.kind === "aggregate" ? target.params : { all_workspaces: true },
    serverFilters,
    allProjects
  );

  const names = new Map(registeredWorkspaces.map(workspace => [workspace.id, workspace.name]));
  const projectOf = (id: string | undefined) => workspaceBadgeIdentity(id, names);
  const batchEntries = loopCatalogBatchEntries(batchQuery.entries);
  const aggregate = allProjects ? loopsCatalogAggregate(batchQuery, projectOf) : undefined;

  const updateSearch = (updater: (current: LoopsRouteSearch) => LoopsRouteSearch) => {
    void navigate({
      search: current => updater(current),
      to: "/loops",
    });
  };

  const queryInput = useDebouncedInput({
    externalValue: searchQuery,
    onCommit: nextQuery =>
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(nextQuery) })),
  });

  const setView = (nextView: ListingViewMode) => {
    updateSearch(current => ({
      ...current,
      view: nextView === "rows" ? undefined : nextView,
    }));
  };

  const setFilters = (next: LoopCatalogFilter) => {
    updateSearch(current => ({
      ...current,
      kind: next.kind === "all" ? undefined : next.kind,
      category: next.category ?? undefined,
      status: next.status ?? undefined,
    }));
  };

  const clearFilters = () => {
    queryInput.reset("");
    projectFilter.clear();
    updateSearch(current => ({
      ...current,
      q: undefined,
      kind: undefined,
      category: undefined,
      status: undefined,
    }));
  };

  // A row opens bound to its own project under All projects; elsewhere it keeps the window's.
  const handleRun = (entry: LoopCatalogEntry) => {
    void navigate({
      to: "/loops/$name/run",
      params: { name: entry.name },
      search: allProjects ? itemBindingSearch(entry.workspace_id) : windowSearch,
    });
  };

  const entries = allProjects ? batchEntries : loopsQuery.loops;

  return {
    allProjects,
    aggregate,
    workspaceLabel: loopsWorkspaceLabel(scope, projectOf(workspaceId).name),
    /** Under All profiles definitions resolve for one concrete profile; the toolbar says which. */
    definitionsNote: profile.aggregate ? `Definitions for ${profile.destination}` : null,
    hasActiveFilters: loopsHasActiveFilters(searchQuery, filter, projectFilter.values.length),
    clearFilters,
    filter,
    categoryOptions: loopsCategoryOptions(allProjects, batchEntries, loopsQuery),
    entries,
    facets: allProjects ? undefined : loopsQuery.facets,
    total: allProjects ? batchEntries.length : loopsQuery.total,
    catalogQuery: loopsCatalogQueryState(allProjects, batchQuery, loopsQuery),
    handleRun,
    projectFilter,
    searchQuery: queryInput.draftValue,
    setSearchQuery: queryInput.setDraftValue,
    setFilters,
    setView,
    view,
    /** A data scope is settled: a project, User, or the All projects aggregate. */
    ready: allProjects || workspaceId !== "",
  };
}

export { useLoopsCatalog };
export type { LoopsRouteSearch } from "@/systems/loops";
