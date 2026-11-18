import { hashKey, type QueryClient, type QueryKey } from "@tanstack/react-query";

import type { FilesRootRef } from "../adapters/files-api";
import { filesKeys } from "./query-keys";

/**
 * Listings whose next fetch must bypass the daemon's short-lived status snapshot
 * (`Cache-Control: no-cache`). Expanding a folder reuses the snapshot; an explicit Refresh,
 * a folder Retry, or a root change asks for a fresh one.
 */
const pending = new Set<string>();

/** Marks every listing the root shows right now (active queries) for a fresh read. */
export function requestFreshListings(queryClient: QueryClient, root: FilesRootRef): void {
  const active = queryClient
    .getQueryCache()
    .findAll({ queryKey: filesKeys.entriesRoot(root), type: "active" });
  for (const query of active) pending.add(query.queryHash);
}

/** Marks one listing for a fresh read (a folder Retry). */
export function requestFreshListing(queryKey: QueryKey): void {
  pending.add(hashKey(queryKey));
}

/** Whether this fetch of `queryKey` was asked to be fresh; the request is consumed. */
export function consumeFreshListing(queryKey: QueryKey): boolean {
  return pending.delete(hashKey(queryKey));
}
