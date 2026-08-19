import { useInfiniteQuery, useQuery } from "@tanstack/react-query";

import { loopRequestDetailOptions, loopRequestsOptions } from "../lib/query-options";
import type { LoopRequest, LoopRequestStableFilter } from "../types";

export function useLoopRequests(
  workspaceId: string,
  filters: LoopRequestStableFilter = {},
  enabled = true
) {
  const query = useInfiniteQuery(loopRequestsOptions(workspaceId, filters, enabled));
  const requests: LoopRequest[] = (query.data?.pages ?? []).flatMap(page => page.items);
  return {
    ...query,
    requests,

    pendingCount: query.data?.pages.at(0)?.aggregates.pending ?? 0,
  };
}

export function useLoopRequestDetail(
  workspaceId: string,
  runId: string,
  generation: number,
  nodeId: string,
  itemIndex?: number,
  enabled = true
) {
  return useQuery(
    loopRequestDetailOptions(workspaceId, runId, generation, nodeId, itemIndex, enabled)
  );
}
