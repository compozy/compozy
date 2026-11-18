import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import {
  stableWindowManagerClientId,
  type WindowManagerConfig,
  windowManagerConfigOptions,
} from "@/systems/os";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  windowManagerLayoutOptions,
  windowManagerLayoutProfilesOptions,
} from "../lib/window-manager-layout-query";
import type {
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutState,
} from "../lib/window-manager-layout-types";

export interface LayoutsSettingsData {
  isPending: boolean;
  error: Error | null;
  workspaceId: string;
  /** The profile whose desks this page reads and applies (US-026). */
  profileName: string;
  workspaceName: string | null;
  /** Global window-manager defaults — the page's save-bar baseline. */
  config: WindowManagerConfig | null;
  layout: WindowManagerLayoutState | null;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  retry: () => void;
}

/**
 * Everything the Layouts page reads, in the two scopes it genuinely needs:
 * window behaviour is a global default, while the layout and the saved layouts
 * belong to the active workspace. Shortcuts read through
 * `useShortcutsSettingsData`.
 */
export function useLayoutsSettingsData(): LayoutsSettingsData {
  const workspace = useActiveWorkspace();
  const workspaceId = workspace.activeWorkspaceId ?? "";
  // Layouts read and write one profile's desks, so this surface names the profile
  // it is editing rather than letting the daemon resolve `default` (US-026).
  const profileName = useProfileReadScope().destination;
  const [clientId] = useState(stableWindowManagerClientId);
  const settings = useQuery(windowManagerConfigOptions(null, clientId, profileName));
  const profiles = useQuery(windowManagerLayoutProfilesOptions(workspaceId, profileName));
  const layout = useQuery(windowManagerLayoutOptions(workspaceId, profileName));
  const waitingForWorkspace = !workspace.hasHydrated || workspace.isLoading || workspace.pending;
  const firstError = [settings.error, profiles.error, workspace.error, layout.error]
    .filter(entry => entry instanceof Error)
    .at(0);

  return {
    isPending:
      settings.isPending ||
      waitingForWorkspace ||
      (workspaceId !== "" && (profiles.isPending || layout.isPending)),
    error: firstError ?? null,
    workspaceId,
    profileName,
    workspaceName: workspace.activeWorkspace?.name ?? null,
    config: settings.data ?? null,
    layout: layout.data ?? null,
    profiles: profiles.data ?? [],
    retry: () => {
      void Promise.all([
        settings.refetch(),
        workspace.refetch(),
        ...(workspaceId === "" ? [] : [profiles.refetch(), layout.refetch()]),
      ]);
    },
  };
}
