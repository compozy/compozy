import { queryOptions } from "@tanstack/react-query";

import { fetchWindowManagerSnapshot } from "../adapters/window-manager-api";
import { fetchWindowManagerSettings } from "../adapters/window-manager-settings-api";
import type { WindowManagerSnapshot } from "./window-manager-types";

/**
 * Scope is part of the key: the same route answers with a different keymap per
 * workspace, and a workspace-scoped rebind must never surface as global truth.
 */
export const WINDOW_MANAGER_GLOBAL_SCOPE = "global";

export function windowManagerScopeKey(workspaceId: string | null): string {
  const normalized = workspaceId?.trim() ?? "";
  return normalized === "" ? WINDOW_MANAGER_GLOBAL_SCOPE : normalized;
}

export const windowManagerKeys = {
  all: ["os", "window-manager"] as const,
  configs: () => ["settings", "section", "window-manager"] as const,
  config: (workspaceId: string | null = null, clientId?: string) =>
    [
      ...windowManagerKeys.configs(),
      windowManagerScopeKey(workspaceId),
      clientId?.trim() || null,
    ] as const,
  snapshots: () => [...windowManagerKeys.all, "snapshot"] as const,
  // One arrangement per (workspace, profile): the profile is part of the key or a
  // switch would show the previous profile's desks from cache (US-026).
  snapshot: (workspaceId: string, profile: string) =>
    [...windowManagerKeys.snapshots(), workspaceId.trim(), profile.trim()] as const,
};

export function windowManagerSnapshotOptions(workspaceId: string, profile: string) {
  const normalized = workspaceId.trim();
  const normalizedProfile = profile.trim();
  return queryOptions({
    queryKey: windowManagerKeys.snapshot(normalized, normalizedProfile),
    queryFn: ({ signal }) => fetchWindowManagerSnapshot(normalized, normalizedProfile, signal),
    enabled: normalized !== "" && normalizedProfile !== "",
    staleTime: Number.POSITIVE_INFINITY,
    retry: 1,
  });
}

/**
 * The whole section: the registry-wide command list, the effective keymap, and
 * the aliases in force. Only a workspace-scoped read carries `ext.*` ids and
 * real command titles — the daemon cannot resolve the catalog without one.
 */
export function windowManagerSettingsOptions(workspaceId: string | null = null, clientId?: string) {
  return queryOptions({
    queryKey: windowManagerKeys.config(workspaceId, clientId),
    queryFn: ({ signal }) => fetchWindowManagerSettings({ workspaceId, clientId }, signal),
    staleTime: 15_000,
    refetchInterval: 60_000,
    retry: 1,
  });
}

/** The keymap half of the same cache entry — never a second request. */
export function windowManagerConfigOptions(workspaceId: string | null = null, clientId?: string) {
  return queryOptions({
    ...windowManagerSettingsOptions(workspaceId, clientId),
    select: section => section.config,
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
