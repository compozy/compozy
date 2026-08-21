import { useQueries } from "@tanstack/react-query";

import { loopRequestAttentionOptions } from "../lib/query-options";
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
    if (query?.isLoading === true) loading = true;
    if (stale) disconnected = true;
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
