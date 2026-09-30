import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { useProfileReadScope } from "@/systems/profiles";
import { sessionScopedDetailOptions, useSessionCatalog } from "@/systems/session";

export function useLoopSessionCatalog(workspaceId: string, value: string) {
  const [search, setSearch] = useState("");
  const { params } = useProfileReadScope();
  const catalog = useSessionCatalog(
    workspaceId,
    {
      limit: 100,
      q: search,
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
    search,
    setSearch,
    selected: visibleSelection ?? scopedSelection,
    selectedLoading: value !== "" && !visibleSelection && selectedQuery.isLoading,
  };
}
