import { useEffect } from "react";

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

export interface SessionGroupFolds {
  isCollapsed: (key: string) => boolean;
  toggle: (key: string) => void;
}

/**
 * Which Group-by groups this window folds under one lens (scope, profile, grouping). A list
 * group and a board swimlane with the same key are one fold, so either view moves both.
 */
export function useSessionGroupFolds(
  scope: DataScope | null,
  groupBy: SessionListGroupBy
): SessionGroupFolds {
  const profile = useProfileReadScope();
  const sidebar = useSessionSidebarState();
  const foldLens = JSON.stringify([scope, profileFoldIdentity(profile), groupBy]);
  const fallbackLens = JSON.stringify([scope, `name:${profile.key}`, groupBy]);
  const resolvedProfile = !profile.aggregate && profile.destinationOwner !== null;
  useEffect(() => {
    if (resolvedProfile) {
      sessionSidebarStore.trigger.groupLensResolved({ from: fallbackLens, to: foldLens });
    }
  }, [resolvedProfile, fallbackLens, foldLens]);
  const groupId = (key: string) => `${foldLens}\u0000${key}`;
  return {
    // The commit that resolves the profile still reads the fold stored under its name, so a
    // folded lane does not open and read its columns until `groupLensResolved` moves it.
    isCollapsed: key =>
      sidebar.groupFlag(groupId(key)) ??
      (resolvedProfile ? sidebar.groupFlag(`${fallbackLens}\u0000${key}`) : undefined) ??
      sidebar.groupFolds.has(groupId(key)),
    toggle: key => sidebar.toggleGroup(groupId(key)),
  };
}
