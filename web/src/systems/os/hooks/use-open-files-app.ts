import { use } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import { OsShellContext } from "../contexts/os-shell-context";
import { filesOpenTarget, filesProjectForOpen } from "../lib/files-open-target";

/**
 * Opens or focuses a named project's Files window: a new window follows the daemon's new-window
 * policy (a tab of the focused window), an existing one is focused. Null outside the desktop
 * shell, where there is no window to open.
 */
export function useOpenProjectFiles(): ((workspaceId: string) => void) | null {
  const shell = use(OsShellContext);
  if (shell === null) return null;
  return workspaceId => void shell.coordinator.userOpen(filesOpenTarget(workspaceId));
}

/**
 * Opens or focuses a project's Files window. Without an explicit project it opens the active
 * one: the focused window's project, else the most recently used project, else the first. Null
 * outside the desktop shell, where there is no window to open.
 */
export function useOpenFilesApp(): ((workspaceId?: string | null) => void) | null {
  const shell = use(OsShellContext);
  const { rememberedWorkspace, workspaces } = useActiveWorkspace();
  if (shell === null) return null;
  const fallback = rememberedWorkspace?.id ?? workspaces[0]?.id ?? null;
  return workspaceId => {
    const state = shell.manager.getState();
    const project = workspaceId ?? filesProjectForOpen(state, fallback);
    void shell.coordinator.userOpen(filesOpenTarget(project));
  };
}
