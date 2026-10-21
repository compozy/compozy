import { keepPreviousData, useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useProfileReadScope } from "@/systems/profiles";
import { workspaceSelectorForScope, type DataScope } from "@/systems/workspace";
import { sessionCatalogOptions, sessionFacetsOptions } from "../lib/session-catalog-options";
import type { SessionListFilters } from "../types";

/**
 * One session catalog page for a data scope: a project, User (`@user`), or
 * every project. `null` — an unsettled scope — reads nothing; it never
 * widens to the aggregate.
 */
export function useSessionCatalog(
  scope: DataScope | null,
  filters: SessionListFilters = {},
  enabled = true,
  options: { facets?: boolean } = {}
) {
  const { params } = useProfileReadScope();
  const scoped = {
    ...filters,
    ...(scope === null || scope.kind === "all"
      ? { all_workspaces: true }
      : { workspace_id: workspaceSelectorForScope(scope) }),
    ...params,
  };
  const active = enabled && scope !== null;
  const pageOptions = sessionCatalogOptions(scoped);
  const { q: _search, ...population } = pageOptions.queryKey[4];
  const populationKey = JSON.stringify(population);
  const page = useInfiniteQuery({
    ...pageOptions,
    enabled: active,
    placeholderData: (previousData, previousQuery) => {
      const previousFilters = previousQuery?.queryKey[4];
      if (!previousFilters || typeof previousFilters !== "object") return undefined;
      const { q: _previousSearch, ...previousPopulation } = previousFilters;
      return JSON.stringify(previousPopulation) === populationKey
        ? keepPreviousData(previousData)
        : undefined;
    },
  });
  const withFacets = options.facets === true;
  const facets = useQuery({
    ...sessionFacetsOptions(scoped),
    enabled: active && withFacets,
  });
  return {
    sessions: page.data?.pages[0]?.sessions ?? [],
    facets: facets.data?.facets,
    total: facets.data?.facets.all,
    loading: page.isLoading,
    /** The page shown belongs to a previous search while the current one loads. */
    placeholder: page.isPlaceholderData,
    failed: page.isError || (withFacets && facets.isError),
    next: !page.isPlaceholderData && page.hasNextPage,
    previous: !page.isPlaceholderData && page.hasPreviousPage,
    paging: page.isPlaceholderData || page.isFetchingNextPage || page.isFetchingPreviousPage,
    nextPage: () => {
      if (!page.isFetching && !page.isPlaceholderData)
        void page.fetchNextPage({ cancelRefetch: false }).catch(() => undefined);
    },
    previousPage: () => {
      if (!page.isFetching && !page.isPlaceholderData)
        void page.fetchPreviousPage({ cancelRefetch: false }).catch(() => undefined);
    },
    retry: () => {
      void page.refetch();
      if (withFacets) void facets.refetch();
    },
  };
}
