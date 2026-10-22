import {
  isActiveWorkspaceStoreHydrated,
  useBreadthMode,
  useFocus,
  useSelectedWorkspaceId,
} from "./use-active-workspace-store";
import { useWorkspaces } from "./use-workspaces";
import { resolveActiveWorkspace } from "../lib/active-workspace";
import type { DataScope } from "../lib/destination";
import {
  clearActiveWorkspaceSelection,
  disableAllProjects,
  enableAllProjects,
  focusDestination,
  focusUser,
  setActiveWorkspaceId,
  toggleAllProjects,
} from "../stores/active-workspace-store";
import { useUserHomeDir } from "./use-user-home-dir";

export function useActiveWorkspace(options: { enabled?: boolean } = {}) {
  const selectedWorkspaceId = useSelectedWorkspaceId();
  const requestedBreadthMode = useBreadthMode();
  const requestedFocus = useFocus();
  const query = useWorkspaces(options);
  // Display only (path contraction, session roots); resolution never reads it.
  const userHomeDir = useUserHomeDir(options);
  const resolution = resolveActiveWorkspace({
    workspaces: query.data ?? [],
    breadthMode: requestedBreadthMode,
    focus: requestedFocus,
    // Until the catalog has data, "deleted" and "still loading" are indistinguishable.
    pending: query.data === undefined,
  });

  return {
    ...query,
    ...resolution,
    workspaces: resolution.projectWorkspaces,
    hasWorkspaces: resolution.projectWorkspaces.length > 0,
    hasHydrated: isActiveWorkspaceStoreHydrated(),
    selectedWorkspaceId,
    requestedBreadthMode,
    userHomeDir,
    setActiveWorkspaceId,
    focusUser,
    focusDestination,
    enableAllProjects,
    disableAllProjects,
    toggleAllProjects: () =>
      toggleAllProjects({ breadthMode: resolution.breadthMode, focus: resolution.focus }),
    clearActiveWorkspaceSelection,
  };
}

/**
 * The current data scope: what every surface lists. `null` while a stored
 * project focus is unconfirmed; readers stay disabled until it settles.
 */
export function useDataScope(options: { enabled?: boolean } = {}): DataScope | null {
  return useActiveWorkspace(options).dataScope;
}
