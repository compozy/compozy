/**
 * Canonical source for sidebar/topbar nav count slots. Wraps the process-wide
 * navCountsStore singleton: one SSE channel + one polling timer per process,
 * shared across every consumer mount. See nav-counts-store.ts for the SSE
 * filter list, polling endpoints, stale semantics, and race rules.
 */
import { useEffect, useSyncExternalStore } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import {
  type NavCount,
  type NavCountKey,
  type NavCountsStatus,
  type NavCountsStore,
  getNavCountsStore,
} from "./nav-counts-store";

export type { NavCount, NavCountKey, NavCountsStatus } from "./nav-counts-store";

export interface UseNavCountsResult {
  counts: Partial<Record<NavCountKey, NavCount>>;
  refresh: () => void;
  status: NavCountsStatus;
}

export interface UseNavCountsOptions {
  /** Override the singleton store. Tests inject this to bypass the global. */
  store?: NavCountsStore;
}

export function useNavCounts(options: UseNavCountsOptions = {}): UseNavCountsResult {
  const { activeWorkspaceId } = useActiveWorkspace();
  const store = options.store ?? getNavCountsStore(activeWorkspaceId);
  const counts = useSyncExternalStore(
    store.subscribe,
    () => store.getState().counts,
    () => store.getState().counts
  );
  const status = useSyncExternalStore(
    store.subscribe,
    () => store.getState().status,
    () => store.getState().status
  );
  useEffect(() => store.getState().retainConsumer(), [store]);
  return {
    counts,
    status,
    refresh: store.getState().refresh,
  };
}

export function selectNavCount(
  result: UseNavCountsResult,
  key: NavCountKey | undefined
): NavCount | undefined {
  if (!key) return undefined;
  return result.counts[key];
}
