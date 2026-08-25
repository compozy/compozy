import { useState } from "react";

import { useAgentCreateDialog, useAgents } from "@/systems/agent";
import { useSessionCatalogStreams, useSessionCreateDialogController } from "@/systems/session";

import { publishAttentionEdge, publishOperatorNotification } from "../lib/attention-edge-bus";
import {
  useActiveWorkspace,
  useUserHomeDir,
  useWorkspace,
  useWorktreeCatalogStream,
  useWorktrees,
} from "@/systems/workspace";

/**
 * Desktop-shell view model: the surviving responsibilities of the deleted
 * `use-app-layout` — workspace resolution, session-catalog stream mounts, and
 * the session/agent create dialog wiring (sidebar state is gone with the
 * sidebar).
 */
export function useDesktopShellModel(
  {
    workspaces,
    registeredWorkspaces,
    hasWorkspaces,
    activeWorkspace,
    activeWorkspaceId,
    runtimeWorkspace,
    runtimeWorkspaceId,
    desktopWorkspace,
    desktopWorkspaceId,
    homeWorkspace,
    scope,
    pending,
    chip,
    toggleLocked,
    canDisableGlobal,
    rememberedWorkspace,
    setActiveWorkspaceId,
    toggleGlobalScope,
    isLoading: areWorkspacesLoading,
    isError: workspacesError,
  }: ReturnType<typeof useActiveWorkspace>,
  {
    backgroundStreamsEnabled = true,
    continuityStreamsEnabled = true,
  }: {
    backgroundStreamsEnabled?: boolean;
    continuityStreamsEnabled?: boolean;
  } = {}
) {
  const { data: agents } = useAgents(runtimeWorkspaceId, {
    enabled: runtimeWorkspaceId !== null,
  });
  const projectWorkspaceDetail = useWorkspace(activeWorkspaceId ?? "", {
    enabled: activeWorkspaceId !== null,
  });
  const workspaceAgents = runtimeWorkspaceId === null ? undefined : agents;
  const [isWorkspaceSetupOpen, setWorkspaceSetupOpen] = useState(false);
  const [worktreeCreateWorkspaceId, setWorktreeCreateWorkspaceId] = useState<string | null>(null);
  // Attention edges leave the stream through the module-level bus so the
  // notifier can subscribe without ever reopening this connection.
  const sessionCatalogStreamStatus = useSessionCatalogStreams({
    enabled: continuityStreamsEnabled,
    onAttentionEdge: publishAttentionEdge,
    onOperatorNotification: publishOperatorNotification,
  });

  // One catalog subscription per shell; the query stays the snapshot authority
  // and this only reconciles workspace-qualified invalidations into it.
  const worktreeCatalogStreamStatus = useWorktreeCatalogStream(workspaces, {
    enabled: hasWorkspaces && backgroundStreamsEnabled,
  });
  const activeWorktrees = useWorktrees(activeWorkspaceId, { enabled: activeWorkspaceId !== null });
  const userHomeDir = useUserHomeDir();
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
    desktopWorkspace,
    desktopWorkspaceId,
    homeWorkspace,
    scope,
    pending,
    chip,
    toggleLocked,
    canDisableGlobal,
    rememberedWorkspace,
    workspaceAgents,
    workspaceProfileHints: projectWorkspaceDetail.data?.profile_hints ?? [],
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
    worktreeListing: activeWorktrees.data,
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
