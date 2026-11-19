import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import {
  stableWindowManagerClientId,
  type WindowManagerSettingsSection,
  windowManagerSettingsOptions,
} from "@/systems/os";
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

export interface ShortcutsSettingsData {
  isPending: boolean;
  error: Error | null;
  workspaceId: string;
  workspaceName: string | null;
  clientId: string | undefined;
  /** Workspace-scoped keyboard truth: the registry, the keymap, the aliases. */
  keyboard: WindowManagerSettingsSection | null;
  retry: () => void;
}

/**
 * The keyboard surface reads and writes the active workspace — the only scope
 * where the daemon can enumerate the whole command registry
 * (`internal/settings/window_manager_section.go`).
 */
export function useShortcutsSettingsData(): ShortcutsSettingsData {
  const workspace = useActiveWorkspace();
  const workspaceId = workspace.activeWorkspaceId ?? "";
  const profileName = useProfileReadScope().destination;
  const [clientId] = useState(stableWindowManagerClientId);
  const keyboard = useQuery(
    windowManagerSettingsOptions(workspaceId === "" ? null : workspaceId, clientId, profileName)
  );
  const waitingForWorkspace = !workspace.hasHydrated || workspace.isLoading || workspace.pending;
  const firstError = [keyboard.error, workspace.error]
    .filter(entry => entry instanceof Error)
    .at(0);

  return {
    isPending: keyboard.isPending || waitingForWorkspace,
    error: firstError ?? null,
    workspaceId,
    workspaceName: workspace.activeWorkspace?.name ?? null,
    clientId,
    keyboard: keyboard.data ?? null,
    retry: () => {
      void Promise.all([keyboard.refetch(), workspace.refetch()]);
    },
  };
}
