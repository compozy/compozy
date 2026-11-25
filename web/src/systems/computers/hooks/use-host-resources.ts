import { useContext, useMemo } from "react";
import { QueryClient, QueryClientContext, useQueries } from "@tanstack/react-query";

import { LOCAL_COMPUTER_ID, type ComputerId } from "@/lib/computer-scope";

import { hostResourcesOptions } from "../lib/query-options";
import type { ComputerPayload, HostResourcesResponse } from "../types";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

const HOST_RESOURCES_REFETCH_MS = 10_000;

/**
 * Reads live `/api/host/resources` samples for this computer and every connected enabled
 * remote computer while Settings → Computers or the Run on dialog is open (US-015).
 */
export function useComputersHostResources(
  computers: readonly Pick<ComputerPayload, "id" | "enabled" | "status">[],
  options: { enabled: boolean; includeLocal: boolean }
): ReadonlyMap<ComputerId, HostResourcesResponse> {
  const contextClient = useContext(QueryClientContext);
  const queryClient = contextClient ?? NOOP_QUERY_CLIENT;
  const active = options.enabled && contextClient !== undefined;

  const targetIds = useMemo<ComputerId[]>(() => {
    if (!active) return [];
    const ids: ComputerId[] = [];
    if (options.includeLocal) {
      ids.push(LOCAL_COMPUTER_ID);
    }
    for (const computer of computers) {
      if (
        computer.enabled &&
        (computer.status === "connected" || computer.status === "needs_attention")
      ) {
        ids.push(computer.id);
      }
    }
    return ids;
  }, [active, computers, options.includeLocal]);

  const queries = useQueries(
    {
      queries: targetIds.map(id => ({
        ...hostResourcesOptions(id, active),
        refetchInterval: active ? HOST_RESOURCES_REFETCH_MS : false,
      })),
    },
    queryClient
  );

  return useMemo(() => {
    const map = new Map<ComputerId, HostResourcesResponse>();
    for (let index = 0; index < targetIds.length; index += 1) {
      const id = targetIds[index];
      const sample = queries[index]?.data;
      if (id && sample) {
        map.set(id, sample);
      }
    }
    return map;
  }, [queries, targetIds]);
}
