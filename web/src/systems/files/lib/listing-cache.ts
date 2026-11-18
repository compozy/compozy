import type { QueryClient } from "@tanstack/react-query";

import type { FilesRootRef } from "../adapters/files-api";
import { filesKeys } from "./query-keys";

/**
 * Collapsed folder listings kept per root for instant re-expansion. Older ones are dropped, so
 * a long session that opens hundreds of folders retains a bounded cache (perf audit P-5);
 * `gcTime` additionally drops any idle listing after five minutes.
 */
export const COLLAPSED_LISTING_LIMIT = 20;

/**
 * Drops the oldest idle listings of `root` beyond `COLLAPSED_LISTING_LIMIT`, and returns their
 * listing paths so the tree forgets them too.
 */
export function pruneCollapsedListings(queryClient: QueryClient, root: FilesRootRef): string[] {
  const idle = queryClient
    .getQueryCache()
    .findAll({ queryKey: filesKeys.entriesRoot(root), type: "inactive" })
    .sort((left, right) => right.state.dataUpdatedAt - left.state.dataUpdatedAt);
  const removed: string[] = [];
  for (const query of idle.slice(COLLAPSED_LISTING_LIMIT)) {
    queryClient.removeQueries({ queryKey: query.queryKey, exact: true });
    const path = query.queryKey.at(-1);
    if (typeof path === "string") removed.push(path);
  }
  return removed;
}
