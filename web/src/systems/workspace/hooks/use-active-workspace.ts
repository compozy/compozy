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

/**
 * The active destination resolved against the workspace catalog, plus the
 * catalog's load state and the actions that move the destination.
 */
export function useActiveWorkspace(options: { enabled?: boolean } = {}) {
  const selectedWorkspaceId = useSelectedWorkspaceId();
  const requestedBreadthMode = useBreadthMode();
  const requestedFocus = useFocus();
  const query = useWorkspaces(options);
  const resolution = resolveActiveWorkspace({
    workspaces: query.data ?? [],
    breadthMode: requestedBreadthMode,
    focus: requestedFocus,
    // Until the catalog has data, "deleted" and "still loading" are indistinguishable.
    pending: query.data === undefined,
  });

  return {
    // Named reads only: spreading the query would read every field, so each
    // unchanged catalog poll would re-render every caller across the shell.
    isLoading: query.isLoading,
    isError: query.isError,
    error: query.error,
    refetch: query.refetch,
    ...resolution,
    workspaces: resolution.projectWorkspaces,
    hasWorkspaces: resolution.projectWorkspaces.length > 0,
    hasHydrated: isActiveWorkspaceStoreHydrated(),
    selectedWorkspaceId,
    requestedBreadthMode,
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
