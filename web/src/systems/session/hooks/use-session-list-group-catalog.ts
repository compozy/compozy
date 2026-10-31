import { sessionListGroupFilters, type SessionListGroupSpec } from "../lib/session-list-groups";

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
  view: Pick<SessionListViewModel, "groupFilters" | "sort">
): GroupCatalog {
  const listed = group.total > 0;
  const read = useSessionCatalog(group.scope, sessionListGroupFilters(view, group), listed, {
    facets: false,
    lens: group.lens,
  });
  return listed ? read : settledEmptyCatalog(read);
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
