import { useEffect } from "react";

import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { useProfileReadScope } from "@/systems/profiles";
import type { DataScope } from "@/systems/workspace";

import type { SessionListGroupBy } from "../lib/session-list-preferences";
import { sessionSidebarStore, useSessionSidebarState } from "./use-session-sidebar-state";

/**
 * The profile half of a persisted fold's lens. Folds outlive the page, so they
 * name the profile by its immutable id: a rename keeps them, and a later
 * profile that reuses the old name does not inherit them. Until the profile
 * list resolves the id, the name stands in.
 */
function profileFoldIdentity(profile: {
  aggregate: boolean;
  key: string;
  destinationOwner: { id: string } | null;
}): string {
  if (profile.aggregate) return profile.key;
  return profile.destinationOwner?.id ?? `name:${profile.key}`;
}

export function sessionGroupFoldLens(
  scope: DataScope | null,
  profile: { aggregate: boolean; key: string; destinationOwner: { id: string } | null },
  groupBy: SessionListGroupBy
): string {
  return JSON.stringify([scope, profileFoldIdentity(profile), groupBy]);
}

export function qualifySessionGroupFoldKey(
  foldLens: string,
  key: string,
  computerId?: ComputerId
): string {
  const normalizedComputer = normalizeComputerId(computerId);
  if (!isRemoteComputerId(normalizedComputer)) return `${foldLens}\u0000${key}`;
  if (
    key.startsWith(`project:${normalizedComputer}:`) ||
    key.startsWith(`profile:${normalizedComputer}:`) ||
    key.startsWith("computer:")
  ) {
    return `${foldLens}\u0000${key}`;
  }
  if (key.startsWith("project:")) {
    return `${foldLens}\u0000project:${normalizedComputer}:${key.slice("project:".length)}`;
  }
  if (key.startsWith("profile:")) {
    return `${foldLens}\u0000profile:${normalizedComputer}:${key.slice("profile:".length)}`;
  }
  return `${foldLens}\u0000project:${normalizedComputer}:${key}`;
}

export interface SessionGroupFolds {
  isCollapsed: (key: string) => boolean;
  toggle: (key: string) => void;
}

/**
 * Which Group-by groups this window folds under one lens (scope, profile, grouping). A list
 * group and a board swimlane with the same key are one fold, so either view moves both.
 * Pass `computerId` when reading a remote computer's groups so the same project id on two
 * computers folds independently (`N9`).
 */
export function useSessionGroupFolds(
  scope: DataScope | null,
  groupBy: SessionListGroupBy,
  computerId?: ComputerId
): SessionGroupFolds {
  const profile = useProfileReadScope();
  const sidebar = useSessionSidebarState();
  const normalizedComputer = normalizeComputerId(computerId);
  const remote = isRemoteComputerId(normalizedComputer);
  const foldLens = sessionGroupFoldLens(scope, profile, groupBy);
  const fallbackLens = JSON.stringify([scope, `name:${profile.key}`, groupBy]);
  const resolvedProfile = !profile.aggregate && profile.destinationOwner !== null;
  useEffect(() => {
    if (resolvedProfile && !remote) {
      sessionSidebarStore.trigger.groupLensResolved({ from: fallbackLens, to: foldLens });
    }
  }, [resolvedProfile, remote, fallbackLens, foldLens]);
  const groupId = (key: string) => qualifySessionGroupFoldKey(foldLens, key, computerId);
  return {
    // The commit that resolves the profile still reads the fold stored under its name, so a
    // folded lane does not open and read its columns until `groupLensResolved` moves it.
    isCollapsed: key =>
      sidebar.groupFlag(groupId(key)) ??
      (resolvedProfile && !remote ? sidebar.groupFlag(`${fallbackLens}\u0000${key}`) : undefined) ??
      sidebar.groupFolds.has(groupId(key)),
    toggle: key => sidebar.toggleGroup(groupId(key)),
  };
}
