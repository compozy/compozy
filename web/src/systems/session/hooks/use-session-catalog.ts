import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useProfileReadScope } from "@/systems/profiles";
import { sessionCatalogOptions, sessionFacetsOptions } from "../lib/session-catalog-options";
import type { SessionListFilters } from "../types";

export function useSessionCatalog(
  workspaceId: string | null,
  filters: SessionListFilters = {},
  enabled = true,
  options: { facets?: boolean } = {}
) {
  const { params } = useProfileReadScope();
  const scoped = {
    ...filters,
    ...(workspaceId ? { workspace_id: workspaceId } : { all_workspaces: true }),
    ...params,
  };
  const page = useInfiniteQuery({ ...sessionCatalogOptions(scoped), enabled });
  const facets = useQuery({
    ...sessionFacetsOptions(scoped),
    enabled: enabled && options.facets !== false,
  });
  return {
    sessions: page.data?.pages[0]?.sessions ?? [],
    facets: facets.data?.facets,
    total: facets.data?.facets.all,
    loading: page.isLoading,
    failed: page.isError || (options.facets !== false && facets.isError),
    next: page.hasNextPage,
    previous: page.hasPreviousPage,
    paging: page.isFetchingNextPage || page.isFetchingPreviousPage,
    nextPage: () => {
      if (!page.isFetching)
        void page.fetchNextPage({ cancelRefetch: false }).catch(() => undefined);
    },
    previousPage: () => {
      if (!page.isFetching)
        void page.fetchPreviousPage({ cancelRefetch: false }).catch(() => undefined);
    },
    retry: () => {
      void page.refetch();
      if (options.facets !== false) void facets.refetch();
    },
  };
}
