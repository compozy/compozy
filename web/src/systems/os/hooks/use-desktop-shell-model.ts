import { useState } from "react";

import { useAgentCreateDialog, useAgents } from "@/systems/agent";
import { useSessionCatalogStreams, useSessionCreateDialog } from "@/systems/session";
import { useActiveWorkspace, useWorkspace } from "@/systems/workspace";
import { useSyncUserHomeDir } from "@/systems/workspace/hooks/use-sync-user-home-dir";

/**
 * Desktop-shell view model: the surviving responsibilities of the deleted
 * `use-app-layout` — workspace resolution, session-catalog stream mounts, and
 * the session/agent create dialog wiring (sidebar state is gone with the
 * sidebar).
 */
export function useDesktopShellModel() {
  useSyncUserHomeDir();
  const {
    workspaces,
    hasWorkspaces,
    activeWorkspace,
    activeWorkspaceId,
    setActiveWorkspaceId,
    isLoading: areWorkspacesLoading,
    isError: workspacesError,
  } = useActiveWorkspace();
  const { data: agents } = useAgents(activeWorkspaceId, {
    enabled: activeWorkspaceId !== null,
  });
  const activeWorkspaceDetail = useWorkspace(activeWorkspaceId ?? "", {
    enabled: activeWorkspaceId !== null,
  });
  const workspaceAgents = activeWorkspaceId === null ? undefined : agents;
  const [isWorkspaceSetupOpen, setWorkspaceSetupOpen] = useState(false);
  const sessionCatalogStreamStatus = useSessionCatalogStreams(workspaces, {
    enabled: hasWorkspaces,
  });
  const sessionCreate = useSessionCreateDialog({
    agents: workspaceAgents,
    activeWorkspace,
  });
  const agentCreate = useAgentCreateDialog({
    activeWorkspace,
    workspaceProviders: activeWorkspaceDetail.data?.providers ?? [],
    workspaceProvidersLoading: activeWorkspaceId !== null && activeWorkspaceDetail.isLoading,
    workspaceProvidersError: activeWorkspaceDetail.error
      ? describeWorkspaceProviderError(activeWorkspaceDetail.error)
      : null,
  });

  return {
    workspaces,
    hasWorkspaces,
    activeWorkspace,
    activeWorkspaceId,
    setActiveWorkspaceId,
    areWorkspacesLoading,
    workspacesError,
    sessionCatalogStreamStatus,
    isWorkspaceSetupOpen,
    setWorkspaceSetupOpen,
    openWorkspaceSetup: () => setWorkspaceSetupOpen(true),
    sessionCreate,
    agentCreate,
  };
}

export type DesktopShellModel = ReturnType<typeof useDesktopShellModel>;

function describeWorkspaceProviderError(error: unknown): string {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return "Unable to load workspace providers.";
}
