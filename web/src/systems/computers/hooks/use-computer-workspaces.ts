import { useContext } from "react";
import { QueryClient, QueryClientContext, useQuery } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";
import { computerWorkspacesOptions } from "@/systems/workspace";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

/**
 * Queries projects/workspaces on a specific computer with key `["computer", id, "workspaces"]` (UT-081).
 */
export function useComputerWorkspaces(computerId: ComputerId, enabled = true) {
  const contextClient = useContext(QueryClientContext);
  return useQuery(
    computerWorkspacesOptions(computerId, enabled && contextClient !== undefined),
    contextClient ?? NOOP_QUERY_CLIENT
  );
}
