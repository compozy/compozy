import { useProfileReadScope } from "@/systems/profiles";
import { useInfiniteQuery } from "@tanstack/react-query";

import { loopNodeInventoryOptions } from "../lib/query-options";
import type { LoopNodeInventoryStableFilter } from "../lib/query-keys";
import type { LoopNodeInventoryItem } from "../types";

/**
 * One workspace node-inventory view (`GET /loop-nodes?state=…`).
 *
 * Boundary contract: the daemon owns scope (workspace path param), filtering
 * (`loop`, `run_id`), ordering, and the page cut (`cursor`/`limit`). It publishes
 * NO totals, so this hook exposes `loadedCount` and `hasMore` and deliberately
 * offers no `total` — a loaded page count is not a population count, and
 * presenting it as one would be the exact completeness lie the catalog contract
 * forbids. The envelope stays in the query cache; flattening happens here, at
 * the view-model read boundary, and nowhere earlier.
 */
export interface LoopNodeInventoryView {
  items: LoopNodeInventoryItem[];
  /** Rows currently loaded — never a population total. */
  loadedCount: number;
  /** True while the server still holds a continuation cursor. */
  hasMore: boolean;
  isLoading: boolean;
  /**
   * No rows yet and no failure — including a read that has not started (a window outside the
   * live-data budget). The inventory shows its skeleton for this, never "Nothing is waiting".
   */
  isPending: boolean;
  isFetchingNextPage: boolean;
  isError: boolean;
  error: Error | null;
  fetchNextPage: () => void;
  refetch: () => void;
}

export function useLoopNodeInventory(
  workspaceId: string,
  filters: LoopNodeInventoryStableFilter,
  enabled = true
): LoopNodeInventoryView {
  const { params } = useProfileReadScope();
  const query = useInfiniteQuery(
    loopNodeInventoryOptions(workspaceId, { ...filters, ...params }, enabled)
  );
  const items = query.data?.pages.flatMap(page => page.items) ?? [];
  // While a loop or run filter is loading, the rows are the previous ones; their cursor belongs
  // to the previous filter, so paging waits for the new first page.
  const paging = !query.isPlaceholderData;
  return {
    items,
    loadedCount: items.length,
    hasMore: paging && query.hasNextPage,
    isLoading: query.isLoading,
    isPending: query.isPending,
    isFetchingNextPage: query.isFetchingNextPage,
    isError: query.isError,
    error: query.error,
    fetchNextPage: () => {
      if (paging && query.hasNextPage && !query.isFetchingNextPage) void query.fetchNextPage();
    },
    refetch: () => {
      void query.refetch();
    },
  };
}
