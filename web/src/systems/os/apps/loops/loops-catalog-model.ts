import type {
  LoopCatalogAggregate,
  LoopCatalogEntry,
  LoopCatalogFilter,
  LoopsRouteSearch,
  useLoops,
  useLoopsAggregate,
} from "@/systems/loops";
import { DESTINATION_COPY, type DataScope, type WorkspaceBadgeIdentity } from "@/systems/workspace";

type LoopsQuery = ReturnType<typeof useLoops>;
type LoopsBatchQuery = ReturnType<typeof useLoopsAggregate>;

/** The catalog filter encoded in the route search. */
export function loopCatalogFilterFromSearch(search: LoopsRouteSearch): LoopCatalogFilter {
  return {
    kind: search.kind ?? "all",
    category: search.category ?? null,
    status: search.status ?? null,
  };
}

/** The filter as the catalog routes spell it. */
export function loopsServerFilters(filter: LoopCatalogFilter, searchQuery: string) {
  const kinds: Partial<Record<LoopCatalogFilter["kind"], "read_only" | "workspace">> = {
    "read-only": "read_only",
    workspace: "workspace",
  };
  return {
    category: filter.category ?? undefined,
    kind: kinds[filter.kind],
    q: searchQuery,
    sort: "name" as const,
    status: filter.status ?? undefined,
  };
}

/** Per-project errors, truncation and the badge resolver of the All projects batch. */
export function loopsCatalogAggregate(
  batchQuery: LoopsBatchQuery,
  projectOf: (id: string | undefined) => WorkspaceBadgeIdentity
): LoopCatalogAggregate {
  return {
    projectOf,
    errors: batchQuery.errors.map(error => ({
      workspaceId: error.workspace_id ?? "",
      label: projectOf(error.workspace_id).name,
      message: error.message,
    })),
    truncated: batchQuery.truncated,
    onRetry: () => void batchQuery.refetch(),
  };
}

/** One loading/paging contract for the paged project catalog and the bounded batch. */
export function loopsCatalogQueryState(
  allProjects: boolean,
  batchQuery: LoopsBatchQuery,
  loopsQuery: LoopsQuery
) {
  if (allProjects) {
    return {
      isLoading: batchQuery.isLoading,
      error: batchQuery.error,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: () => undefined,
      retry: () => void batchQuery.refetch(),
    };
  }
  return {
    isLoading: loopsQuery.isLoading,
    error: loopsQuery.error,
    hasNextPage: loopsQuery.hasNextPage,
    isFetchingNextPage: loopsQuery.isFetchingNextPage,
    fetchNextPage: () => void loopsQuery.fetchNextPage(),
    retry: () => void loopsQuery.refetch(),
  };
}

/** The scope name shown by the catalog: All projects, User, or the project. */
export function loopsWorkspaceLabel(scope: DataScope | null, projectName: string): string {
  if (scope?.kind === "all") return DESTINATION_COPY.allProjects;
  if (scope?.kind === "user") return DESTINATION_COPY.user;
  return projectName;
}

export function loopsHasActiveFilters(
  searchQuery: string,
  filter: LoopCatalogFilter,
  projectFilterCount: number
): boolean {
  if (searchQuery.trim() !== "" || projectFilterCount > 0) return true;
  return filter.kind !== "all" || filter.category !== null || filter.status !== null;
}

/** Categories offered by the filter. The batch carries no facets, so it reads its own rows. */
export function loopsCategoryOptions(
  allProjects: boolean,
  batchEntries: readonly LoopCatalogEntry[],
  loopsQuery: LoopsQuery
): string[] {
  const categories = allProjects
    ? batchEntries.flatMap(entry => entry.catalog?.category?.trim() || [])
    : Object.keys(loopsQuery.facets?.categories ?? {});
  return [...new Set(categories)].sort();
}
