import { useQueries, useQuery } from "@tanstack/react-query";

import { loopRequestAttentionOptions, loopRunRequestCountsOptions } from "../lib/query-options";
import type { LoopRequest } from "../types";

export interface LoopRequestAttentionItem {
  request: LoopRequest;
  workspaceId: string;
  workspaceLabel: string;
  stale: boolean;
}

export interface LoopRequestAttention {
  pendingCount: number;
  items: LoopRequestAttentionItem[];
  disconnected: boolean;
  loading: boolean;
}

interface AttentionWorkspace {
  id: string;
  name: string;
}

export function useLoopRequestAttention(
  workspaces: readonly AttentionWorkspace[],
  enabled = true,
  poll = true
): LoopRequestAttention {
  const queries = useQueries({
    queries: workspaces.map(workspace =>
      loopRequestAttentionOptions(workspace.id, enabled, poll ? 15_000 : false)
    ),
  });

  let pendingCount = 0;
  let disconnected = false;
  let loading = false;
  const items: LoopRequestAttentionItem[] = [];

  workspaces.forEach((workspace, index) => {
    const query = queries[index];
    const stale = query?.isError === true;
    loading ||= query?.isLoading === true;
    disconnected ||= stale;
    if (!stale) pendingCount += query?.data?.aggregates.pending ?? 0;
    for (const request of query?.data?.items ?? []) {
      items.push({
        request,
        workspaceId: workspace.id,
        workspaceLabel: workspace.name,
        stale,
      });
    }
  });

  return { pendingCount, items, disconnected, loading };
}

export function useLoopRunPendingRequestCounts(
  workspaceId: string,
  runIds: readonly string[],
  enabled = true
): ReadonlyMap<string, number> {
  const query = useQuery(loopRunRequestCountsOptions(workspaceId, enabled && runIds.length > 0));
  const counts = new Map<string, number>();
  if (query.isError) return counts;
  for (const runId of runIds) {
    const count = query.data?.[runId] ?? 0;
    if (count > 0) counts.set(runId, count);
  }
  return counts;
}
