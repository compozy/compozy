import { useQuery } from "@tanstack/react-query";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useProfileReadScope } from "@/systems/profiles";
import { sessionScopedDetailOptions, useSessionCatalog } from "@/systems/session";

export function useLoopSessionCatalog(workspaceId: string, value: string) {
  const {
    draftValue: search,
    committedValue: remoteSearch,
    setDraftValue: setSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  const { params } = useProfileReadScope();
  const catalog = useSessionCatalog(
    workspaceId,
    {
      limit: 100,
      q: remoteSearch,
      search_fields: "title_agent",
    },
    workspaceId !== "",
    { facets: false }
  );
  const visibleSelection = catalog.sessions.find(session => session.id === value);
  const selectedQuery = useQuery(
    sessionScopedDetailOptions(value, params, {
      enabled: workspaceId !== "" && value !== "" && !visibleSelection,
      liveTail: false,
    })
  );
  const scopedSelection =
    selectedQuery.data?.workspace_id === workspaceId ? selectedQuery.data : undefined;
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
