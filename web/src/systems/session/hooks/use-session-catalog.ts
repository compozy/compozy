import {
  keepPreviousData,
  useInfiniteQuery,
  useQuery,
  type InfiniteQueryObserverResult,
  type InfiniteData,
} from "@tanstack/react-query";
import { useProfileReadScope, type ProfileScopeParams } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";
import { sessionCatalogOptions, sessionFacetsOptions } from "../lib/session-catalog-options";
import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionCatalogPageResponse, SessionListFilters } from "../types";

/**
 * One session catalog page for a data scope: a project, User (`@user`), or
 * every project. `null` — an unsettled scope — reads nothing; it never
 * widens to the aggregate. `lens` overrides the read profile lens (a
 * Group-by profile group reads exactly one profile).
 */
export function useSessionCatalog(
  scope: DataScope | null,
  filters: SessionListFilters = {},
  enabled = true,
  options: { facets?: boolean; lens?: ProfileScopeParams } = {}
) {
  const { params } = useProfileReadScope();
  const scoped = sessionCatalogScopedFilters(scope, filters, options.lens ?? params);
  const active = enabled && scope !== null;
  const pageOptions = sessionCatalogOptions(scoped);
  const { q: _search, ...population } = pageOptions.queryKey[6];
  const populationKey = JSON.stringify(population);
  const page = useInfiniteQuery({
    ...pageOptions,
    enabled: active,
    placeholderData: (previousData, previousQuery) => {
      const previousFilters = previousQuery?.queryKey[6];
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
  const catalog = sessionCatalogPageView(page);
  return {
    ...catalog,
    facets: facets.data?.facets,
    total: facets.data?.facets.all ?? catalog.total,
    failed: page.isError || (withFacets && facets.isError),
    retry: () => {
      void page.refetch();
      if (withFacets) void facets.refetch();
    },
  };
}

export function sessionCatalogPageView(
  page: InfiniteQueryObserverResult<InfiniteData<SessionCatalogPageResponse>>
) {
  return {
    sessions: page.data?.pages[0]?.sessions ?? [],
    total: page.data?.pages[0]?.page.total ?? undefined,
    /** Running sessions under the same scope and filters as `total` (#1926). */
    working: page.data?.pages[0]?.page.working ?? undefined,
    /** Sessions needing you under the same scope and filters as `total`. */
    needsYou: page.data?.pages[0]?.page.needs_you ?? undefined,
    loading: page.isLoading,
    /** The page shown belongs to a previous search while the current one loads. */
    placeholder: page.isPlaceholderData,
    failed: page.isError,
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
    },
  };
}
