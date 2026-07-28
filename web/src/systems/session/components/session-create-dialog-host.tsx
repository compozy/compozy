import type { AgentPayload } from "@/systems/agent";
import type { WorkspacePayload } from "@/systems/workspace";

import {
  useSessionCreateDialogViewModel,
  type SessionCreateDialogController,
} from "../hooks/use-session-create-dialog";
import { SessionCreateDialog } from "./session-create-dialog";

interface SessionCreateDialogHostProps extends SessionCreateDialogController {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
}

/** Isolates session-create selectors from the desktop chrome. */
export function SessionCreateDialogHost({
  agents,
  activeWorkspace,
  store,
}: SessionCreateDialogHostProps) {
  const sessionCreate = useSessionCreateDialogViewModel({ agents, activeWorkspace }, store);

  return (
    <SessionCreateDialog
      agents={sessionCreate.agents}
      catalogError={sessionCreate.catalogError}
      catalogLoading={sessionCreate.catalogLoading}
      catalogLoaded={sessionCreate.catalogLoaded}
      catalogRefreshError={sessionCreate.catalogRefreshError}
      catalogRefreshing={sessionCreate.catalogRefreshing}
      catalogStale={sessionCreate.catalogStale}
      hasProviderOptions={sessionCreate.hasProviderOptions}
      isSubmitting={sessionCreate.isSubmitting}
      mode={sessionCreate.mode}
      networkParticipation={sessionCreate.networkParticipation}
      onAgentChange={sessionCreate.onAgentChange}
      onCatalogRefresh={sessionCreate.refreshCatalog}
      onModeChange={sessionCreate.onModeChange}
      onNetworkParticipationChange={sessionCreate.onNetworkParticipationChange}
      onOpenChange={sessionCreate.onOpenChange}
      onOpenProviderSettings={sessionCreate.openProviderSettings}
      onPromptChange={sessionCreate.onPromptChange}
      onRuntimeChange={sessionCreate.onRuntimeChange}
      onSessionNameChange={sessionCreate.onSessionNameChange}
      onSubmit={sessionCreate.submit}
      onWorkspaceChange={sessionCreate.onWorkspaceChange}
      onWorkspacePathChange={sessionCreate.onWorkspacePathChange}
      open={sessionCreate.open}
      promptValue={sessionCreate.promptValue}
      providersError={sessionCreate.providersError}
      providersLoading={sessionCreate.providersLoading}
      runtimeModels={sessionCreate.runtimeModels}
      runtimeProviders={sessionCreate.runtimeProviders}
      runtimeValue={sessionCreate.runtimeValue}
      selectedAgentName={sessionCreate.selectedAgentName}
      sessionName={sessionCreate.sessionName}
      submitError={sessionCreate.submitError}
      userHomeDir={sessionCreate.userHomeDir}
      workspace={sessionCreate.workspace}
      workspaceId={sessionCreate.workspaceId}
      workspacePath={sessionCreate.workspacePath}
      workspaces={sessionCreate.workspaces}
    />
  );
}
