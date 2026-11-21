import { sessionListGroupFilters, type SessionListGroupSpec } from "../lib/session-list-groups";

import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionFoldersState } from "./use-session-folders";
import { useSessionListPartitions } from "./use-session-list-partitions";
import { useSessionSidebarState } from "./use-session-sidebar-state";

import { useSessionCatalog } from "./use-session-catalog";
import type { SessionListViewModel } from "./use-session-list-view";

type GroupCatalog = ReturnType<typeof useSessionCatalog>;
type CatalogScope = Omit<Pick<SessionListGroupSpec, "scope" | "lens" | "pair">, "scope"> & {
  scope: SessionListGroupSpec["scope"] | null;
};
type CatalogView = Pick<SessionListViewModel, "groupFilters" | "sort" | "shelf">;

/**
 * A pair group's own page. A pair with no sessions in view skips the read but keeps its
 * folders, so a folder created from its + shows at once (#1085). A disabled read still
 * holds its last page or error, so the group reads as settled and empty instead.
 */
export function useSessionListGroupCatalog(
  group: CatalogScope & Pick<SessionListGroupSpec, "total">,
  view: CatalogView,
  folders: SessionFoldersState,
  search: string
) {
  const listed = group.total > 0;
  const result = useSessionListContainerCatalog(group, view, folders, listed, search);
  return listed ? result : { catalog: settledEmptyCatalog(result.catalog), partitions: undefined };
}

export function useSessionListContainerCatalog(
  group: CatalogScope,
  view: CatalogView,
  folders: SessionFoldersState,
  enabled: boolean,
  search: string
) {
  const { folderFolds } = useSessionSidebarState();
  const filters = sessionListGroupFilters(view, group, folders.layout !== undefined);
  const partitions = useSessionListPartitions(
    sessionCatalogScopedFilters(group.scope, filters, group.lens),
    folders.layout?.folders,
    folderFolds,
    enabled,
    search.trim() !== ""
  );
  const read = useSessionCatalog(group.scope, filters, enabled && !partitions && !folders.loading, {
    facets: false,
    lens: group.lens,
  });
  const catalog = partitions?.catalog ?? {
    ...read,
    loading: read.loading || folders.loading === true,
  };
  return { catalog, partitions };
}

/** A group catalog with nothing in view: no rows, nothing loading, failed, or paging. */
function settledEmptyCatalog(catalog: GroupCatalog): GroupCatalog {
  return {
    ...catalog,
    sessions: [],
    loading: false,
    placeholder: false,
    failed: false,
    next: false,
    previous: false,
    paging: false,
  };
}
