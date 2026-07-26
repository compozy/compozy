import { queryOptions } from "@tanstack/react-query";

import {
  fetchWindowManagerConfig,
  fetchWindowManagerSnapshot,
} from "../adapters/window-manager-api";
import type { WindowManagerSnapshot } from "./window-manager-types";

export const windowManagerKeys = {
  all: ["os", "window-manager"] as const,
  config: () => ["settings", "section", "window-manager"] as const,
  snapshots: () => [...windowManagerKeys.all, "snapshot"] as const,
  snapshot: (workspaceId: string) =>
    [...windowManagerKeys.snapshots(), workspaceId.trim()] as const,
};

export function windowManagerSnapshotOptions(workspaceId: string) {
  const normalized = workspaceId.trim();
  return queryOptions({
    queryKey: windowManagerKeys.snapshot(normalized),
    queryFn: ({ signal }) => fetchWindowManagerSnapshot(normalized, signal),
    enabled: normalized !== "",
    staleTime: Number.POSITIVE_INFINITY,
    retry: 1,
  });
}

export function windowManagerConfigOptions() {
  return queryOptions({
    queryKey: windowManagerKeys.config(),
    queryFn: ({ signal }) => fetchWindowManagerConfig(signal),
    staleTime: 15_000,
    refetchInterval: 60_000,
    retry: 1,
  });
}

/** Atomic monotonic replacement; equal-revision command echoes are ignored. */
export function reconcileWindowManagerSnapshot(
  current: WindowManagerSnapshot | undefined,
  incoming: WindowManagerSnapshot
): WindowManagerSnapshot {
  if (current === undefined) return incoming;
  if (current.workspaceId !== incoming.workspaceId) return current;
  return incoming.revision > current.revision ? incoming : current;
}
