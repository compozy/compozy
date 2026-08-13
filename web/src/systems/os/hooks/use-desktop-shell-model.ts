import { useState } from "react";

import { useAgentCreateDialog, useAgents } from "@/systems/agent";
import { useSessionCatalogStreams, useSessionCreateDialogController } from "@/systems/session";
import {
  useActiveWorkspace,
  useUserHomeDir,
  useWorkspace,
  useWorktreeCatalogStream,
  useWorktreeListings,
} from "@/systems/workspace";

/**
 * Desktop-shell view model: the surviving responsibilities of the deleted
 * `use-app-layout` — workspace resolution, session-catalog stream mounts, and
 * the session/agent create dialog wiring (sidebar state is gone with the
 * sidebar).
 */
export function useDesktopShellModel() {
  const {
    workspaces,
    registeredWorkspaces,
    hasWorkspaces,
    activeWorkspace,
    activeWorkspaceId,
    runtimeWorkspace,
    runtimeWorkspaceId,
    homeWorkspace,
    scope,
    pending,
    chip,
    toggleLocked,
    canDisableGlobal,
    deletionNotice,
    rememberedWorkspace,
    setActiveWorkspaceId,
    toggleGlobalScope,
    isLoading: areWorkspacesLoading,
    isError: workspacesError,
  } = useActiveWorkspace();
  const { data: agents } = useAgents(runtimeWorkspaceId, {
    enabled: runtimeWorkspaceId !== null,
  });
  const projectWorkspaceDetail = useWorkspace(activeWorkspaceId ?? "", {
    enabled: activeWorkspaceId !== null,
  });
  const workspaceAgents = runtimeWorkspaceId === null ? undefined : agents;
  const [isWorkspaceSetupOpen, setWorkspaceSetupOpen] = useState(false);
  const [worktreeCreateWorkspaceId, setWorktreeCreateWorkspaceId] = useState<string | null>(null);
  const sessionCatalogStreamStatus = useSessionCatalogStreams(registeredWorkspaces, {
    enabled: registeredWorkspaces.length > 0,
  });

  // One catalog subscription per shell; the query stays the snapshot authority
  // and this only reconciles workspace-qualified invalidations into it.
  const worktreeCatalogStreamStatus = useWorktreeCatalogStream(workspaces, {
    enabled: hasWorkspaces,
  });
  // The switcher, the menubar menu, and the overview all list every workspace,
  // so every authorized workspace's worktrees are loaded — scoping to the active
  // one would silently drop the rest of the tree.
  const worktreesByWorkspace = useWorktreeListings(workspaces, { enabled: hasWorkspaces });
  const userHomeDir = useUserHomeDir();
  const activeWorktreeListing = activeWorkspaceId
    ? worktreesByWorkspace[activeWorkspaceId]
    : undefined;
  const sessionCreate = useSessionCreateDialogController();
  const agentCreate = useAgentCreateDialog({
    activeWorkspace,
    workspaceProviders: projectWorkspaceDetail.data?.providers ?? [],
    workspaceProvidersLoading: activeWorkspaceId !== null && projectWorkspaceDetail.isLoading,
    workspaceProvidersError: projectWorkspaceDetail.error
      ? describeWorkspaceProviderError(projectWorkspaceDetail.error)
      : null,
  });

  return {
    workspaces,
    registeredWorkspaces,
    hasWorkspaces,
    activeWorkspace,
    activeWorkspaceId,
    runtimeWorkspace,
    runtimeWorkspaceId,
    homeWorkspace,
    scope,
    pending,
    chip,
    toggleLocked,
    canDisableGlobal,
    deletionNotice,
    rememberedWorkspace,
    workspaceAgents,
    setActiveWorkspaceId,
    toggleGlobalScope,
    areWorkspacesLoading,
    workspacesError,
    sessionCatalogStreamStatus,
    isWorkspaceSetupOpen,
    setWorkspaceSetupOpen,
    openWorkspaceSetup: () => setWorkspaceSetupOpen(true),
    sessionCreate,
    agentCreate,
    userHomeDir,
    worktreeCatalogStreamStatus,
    worktreesByWorkspace,
    worktreeListing: activeWorktreeListing,
    worktreeCreateWorkspaceId,
    setWorktreeCreateWorkspaceId,
    openWorktreeCreate: (workspaceId: string) => setWorktreeCreateWorkspaceId(workspaceId),
  };
}

export type DesktopShellModel = ReturnType<typeof useDesktopShellModel>;

function describeWorkspaceProviderError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "Unable to load workspace providers.";
}
