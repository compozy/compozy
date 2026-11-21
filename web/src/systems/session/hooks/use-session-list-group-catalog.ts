import { sessionListGroupFilters, type SessionListGroupSpec } from "../lib/session-list-groups";

import { sessionCatalogScopedFilters } from "../lib/session-catalog-scope";
import type { SessionFoldersState } from "./use-session-folders";
import { useSessionListPartitions } from "./use-session-list-partitions";
import { useSessionSidebarState } from "./use-session-sidebar-state";

import { useSessionCatalog } from "./use-session-catalog";
import type { SessionListViewModel } from "./use-session-list-view";

type GroupCatalog = ReturnType<typeof useSessionCatalog>;

/**
 * A pair group's own page. A pair with no sessions in view skips the read but keeps its
 * folders, so a folder created from its + shows at once (#1085). A disabled read still
 * holds its last page or error, so the group reads as settled and empty instead.
 */
export function useSessionListGroupCatalog(
  group: Pick<SessionListGroupSpec, "scope" | "lens" | "pair" | "total">,
  view: Pick<SessionListViewModel, "groupFilters" | "sort" | "shelf">,
  folders: SessionFoldersState,
  filtering: boolean
) {
  const listed = group.total > 0;
  const { folderFolds } = useSessionSidebarState();
  const partitions = useSessionListPartitions(
    sessionCatalogScopedFilters(
      group.scope,
      sessionListGroupFilters(view, group, true),
      group.lens
    ),
    folders.layout?.folders,
    folderFolds,
    listed,
    filtering
  );
  const read = useSessionCatalog(
    group.scope,
    sessionListGroupFilters(view, group, folders.layout !== undefined),
    listed && !partitions && !folders.loading,
    { facets: false, lens: group.lens }
  );
  const fallback = listed
    ? { ...read, loading: read.loading || folders.loading === true }
    : settledEmptyCatalog(read);
  return { catalog: partitions?.catalog ?? fallback, partitions };
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
