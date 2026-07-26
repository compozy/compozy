import { useNavigate } from "@tanstack/react-router";

import type { ListingViewMode } from "@agh/ui";

import { useLoops } from "@/systems/loops";
import type {
  LoopCatalogEntry,
  LoopCatalogFilter,
  LoopKindFilter,
  LoopStatusFilter,
} from "@/systems/loops";
import {
  parseLoopCategoryFilter,
  parseLoopKindFilter,
  parseLoopStatusFilter,
} from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";
import { normalizeListingSearchValue, parseListingView } from "@/lib/listing-search";

export interface LoopsRouteSearch {
  q?: string;
  view?: ListingViewMode;
  kind?: Exclude<LoopKindFilter, "all">;
  category?: string;
  status?: LoopStatusFilter;
}

/** View-model for the Loops catalog route: URL state, data, bindings, and Run launch. */
function useLoopsCatalog(search: LoopsRouteSearch = {}) {
  const { activeWorkspaceId, activeWorkspace } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
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

  const setKindFilter = (next: LoopKindFilter) => {
    updateSearch(current => ({
      ...current,
      kind: next === "all" ? undefined : next,
    }));
  };

  const setCategoryFilter = (next: string | null) => {
    updateSearch(current => ({
      ...current,
      category: next ?? undefined,
    }));
  };

  const setStatusFilter = (next: LoopStatusFilter | null) => {
    updateSearch(current => ({
      ...current,
      status: next ?? undefined,
    }));
  };

  const clearFilters = () => {
    updateSearch(current => ({
      ...current,
      q: undefined,
      kind: undefined,
      category: undefined,
      status: undefined,
    }));
  };

  const handleRefresh = () => {
    void loopsQuery.refetch();
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
    handleRefresh,
    handleRun,
    loopsQuery,
    searchQuery,
    setCategoryFilter,
    setKindFilter,
    setSearchQuery,
    setStatusFilter,
    setView,
    view,
    workspaceId,
  };
}

export { parseLoopCategoryFilter, parseLoopKindFilter, parseLoopStatusFilter, useLoopsCatalog };

export function validateLoopsSearch(search: Record<string, unknown>): LoopsRouteSearch {
  return {
    category: parseLoopCategoryFilter(search.category),
    kind: parseLoopKindFilter(search.kind),
    q: normalizeListingSearchValue(search.q),
    status: parseLoopStatusFilter(search.status),
    view: parseListingView(search.view),
  };
}
