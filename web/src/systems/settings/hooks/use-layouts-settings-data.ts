import { useQuery } from "@tanstack/react-query";

import {
  type WindowManagerConfig,
  type WindowManagerSettingsSection,
  windowManagerConfigOptions,
  windowManagerSettingsOptions,
} from "@/systems/os";
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
  workspaceName: string | null;
  /** Global window-manager defaults — the page's save-bar baseline. */
  config: WindowManagerConfig | null;
  /** Workspace-scoped keyboard truth: the registry, the keymap, the aliases. */
  keyboard: WindowManagerSettingsSection | null;
  layout: WindowManagerLayoutState | null;
  profiles: readonly WindowManagerLayoutResourceRecord[];
  retry: () => void;
}

/**
 * Everything the Layouts page reads, in the two scopes it genuinely needs.
 *
 * Window behaviour is a global default, while the keyboard surface reads and
 * writes the active workspace — the only scope where the daemon can enumerate
 * the whole command registry (`internal/settings/window_manager_section.go`).
 */
export function useLayoutsSettingsData(): LayoutsSettingsData {
  const workspace = useActiveWorkspace();
  const workspaceId = workspace.activeWorkspaceId ?? "";
  const settings = useQuery(windowManagerConfigOptions());
  const keyboard = useQuery(windowManagerSettingsOptions(workspaceId === "" ? null : workspaceId));
  const profiles = useQuery(windowManagerLayoutProfilesOptions(workspaceId));
  const layout = useQuery(windowManagerLayoutOptions(workspaceId));
  const waitingForWorkspace = !workspace.hasHydrated || workspace.isLoading;
  const firstError = [settings.error, keyboard.error, profiles.error, workspace.error, layout.error]
    .filter(entry => entry instanceof Error)
    .at(0);

  return {
    isPending:
      settings.isPending ||
      keyboard.isPending ||
      waitingForWorkspace ||
      (workspaceId !== "" && (profiles.isPending || layout.isPending)),
    error: firstError ?? null,
    workspaceId,
    workspaceName: workspace.activeWorkspace?.name ?? null,
    config: settings.data ?? null,
    keyboard: keyboard.data ?? null,
    layout: layout.data ?? null,
    profiles: profiles.data ?? [],
    retry: () => {
      void Promise.all([
        settings.refetch(),
        keyboard.refetch(),
        workspace.refetch(),
        ...(workspaceId === "" ? [] : [profiles.refetch(), layout.refetch()]),
      ]);
    },
  };
}
