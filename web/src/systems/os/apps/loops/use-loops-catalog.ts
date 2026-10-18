import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@compozy/ui";

import type { LoopsRouteSearch } from "@/systems/loops";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { type LoopCatalogEntry, type LoopCatalogFilter, useLoops } from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

/** View-model for the Loops catalog route: URL state, data, bindings, and Run launch. */
function useLoopsCatalog(search: LoopsRouteSearch = {}) {
  const { activeWorkspace, runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId ?? "";
  const navigate = useNavigate({ from: "/loops" });

  const searchQuery = search.q ?? "";
  const view: ListingViewMode = search.view ?? "rows";
  const filter: LoopCatalogFilter = {
    kind: search.kind ?? "all",
    category: search.category ?? null,
    status: search.status ?? null,
  };

  const catalogFilters = {
    category: filter.category ?? undefined,
    kind:
      filter.kind === "read-only"
        ? ("read_only" as const)
        : filter.kind === "workspace"
          ? ("workspace" as const)
          : undefined,
    limit: 50,
    q: searchQuery,
    sort: "name" as const,
    status: filter.status ?? undefined,
  };
  const loopsQuery = useLoops(workspaceId, catalogFilters, workspaceId !== "");

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
    updateSearch(current => ({
      ...current,
      q: undefined,
      kind: undefined,
      category: undefined,
      status: undefined,
    }));
  };

  const handleRun = (entry: LoopCatalogEntry) => {
    void navigate({ to: "/loops/$name/run", params: { name: entry.name } });
  };

  return {
    activeWorkspace,
    hasActiveFilters:
      searchQuery.trim() !== "" ||
      filter.kind !== "all" ||
      filter.category !== null ||
      filter.status !== null,
    clearFilters,
    filter,
    categoryOptions: Object.keys(loopsQuery.facets?.categories ?? {}).sort(),
    facets: loopsQuery.facets,
    handleRun,
    loopsQuery,
    searchQuery: queryInput.draftValue,
    setSearchQuery: queryInput.setDraftValue,
    setFilters,
    setView,
    view,
    workspaceId,
  };
}

export { useLoopsCatalog };
export type { LoopsRouteSearch } from "@/systems/loops";
