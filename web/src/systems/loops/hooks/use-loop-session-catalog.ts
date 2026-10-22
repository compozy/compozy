import { useQuery } from "@tanstack/react-query";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useProfileReadScope } from "@/systems/profiles";
import { sessionScopedDetailOptions, useSessionCatalog } from "@/systems/session";
import { dataScopeFromSelector, ownerDataScope, sameDataScope } from "@/systems/workspace";

/** Session picker for a Loop input: `workspaceId` is the run's selector (a project id or `@user`). */
export function useLoopSessionCatalog(workspaceId: string, value: string) {
  const scope = dataScopeFromSelector(workspaceId);
  const {
    draftValue: search,
    committedValue: remoteSearch,
    setDraftValue: setSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  const { params } = useProfileReadScope();
  const catalog = useSessionCatalog(
    scope,
    {
      limit: 100,
      q: remoteSearch,
      search_fields: "title_agent",
    },
    true,
    { facets: false }
  );
  const visibleSelection = catalog.sessions.find(session => session.id === value);
  const selectedQuery = useQuery(
    sessionScopedDetailOptions(value, params, {
      enabled: scope !== null && value !== "" && !visibleSelection,
      liveTail: false,
    })
  );
  const selected = selectedQuery.data;
  const scopedSelection =
    selected && scope && sameDataScope(ownerDataScope(selected.workspace_id), scope)
      ? selected
      : undefined;
  return {
    ...catalog,
    paging: catalog.paging || search !== remoteSearch,
    next: search === remoteSearch && catalog.next,
    previous: search === remoteSearch && catalog.previous,
    search,
    setSearch,
    selected: visibleSelection ?? scopedSelection,
    selectedLoading: value !== "" && !visibleSelection && selectedQuery.isLoading,
  };
}
