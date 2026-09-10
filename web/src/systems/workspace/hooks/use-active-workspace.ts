import { useEffect } from "react";

import {
  isActiveWorkspaceStoreHydrated,
  useDesktopWorkspaceId,
  useSelectedWorkspaceId,
  useWorkspaceScopeMode,
} from "./use-active-workspace-store";
import { useWorkspaces } from "./use-workspaces";
import { resolveActiveWorkspace } from "../lib/active-workspace";
import {
  activeWorkspaceStore,
  clearActiveWorkspaceSelection,
  disableGlobalScope,
  enableGlobalScope,
  setActiveWorkspaceId,
  toggleGlobalScope,
} from "../stores/active-workspace-store";
import { useDaemonStatus } from "@/systems/status";

export function useActiveWorkspace(options: { enabled?: boolean } = {}) {
  const selectedWorkspaceId = useSelectedWorkspaceId();
  const desktopWorkspaceId = useDesktopWorkspaceId();
  const requestedScope = useWorkspaceScopeMode();
  const query = useWorkspaces(options);
  const status = useDaemonStatus(options);
  const userHomeDir = status.data?.user_home_dir ?? undefined;
  const resolution = resolveActiveWorkspace({
    workspaces: query.data ?? [],
    userHomeDir,
    scope: requestedScope,
    selectedWorkspaceId,
    desktopWorkspaceId,
    // Until both sources have data, "deleted" and "still loading" are indistinguishable.
    pending: query.data === undefined || userHomeDir === undefined,
  });
  const resolvedDesktopWorkspaceId = resolution.desktopWorkspaceId;
  useEffect(() => {
    if (resolvedDesktopWorkspaceId === null) return;
    activeWorkspaceStore.trigger.desktopWorkspaceObserved({
      workspaceId: resolvedDesktopWorkspaceId,
      selectedWorkspaceId,
      previousDesktopWorkspaceId: desktopWorkspaceId,
    });
  }, [desktopWorkspaceId, resolvedDesktopWorkspaceId, selectedWorkspaceId]);

  return {
    ...query,
    ...resolution,
    workspaces: resolution.projectWorkspaces,
    hasWorkspaces: resolution.projectWorkspaces.length > 0,
    hasHydrated: isActiveWorkspaceStoreHydrated(),
    selectedWorkspaceId,
    requestedScope,
    userHomeDir,
    setActiveWorkspaceId,
    enableGlobalScope,
    disableGlobalScope,
    toggleGlobalScope: () =>
      toggleGlobalScope({ scope: resolution.scope, canDisable: resolution.canDisableGlobal }),
    clearActiveWorkspaceSelection,
  };
}
