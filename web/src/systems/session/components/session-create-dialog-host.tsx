import {
  useSessionCreateDialogViewModel,
  type SessionCreateDialogController,
} from "../hooks/use-session-create-dialog";
import { SessionCreateDialog } from "./session-create-dialog";
import type { AgentPayload } from "@/systems/agent";
import type { WorkspacePayload, WorkspaceScopeMode } from "@/systems/workspace";
import { useAggregateDestination } from "@/systems/profiles";

interface SessionCreateDialogHostProps extends SessionCreateDialogController {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
  scope: WorkspaceScopeMode;
  projectWorkspaceId: string | null;
  homeWorkspaceId: string | undefined;
}

/** Isolates session-create selectors from the desktop chrome. */
export function SessionCreateDialogHost({
  agents,
  activeWorkspace,
  scope,
  projectWorkspaceId,
  homeWorkspaceId,
  store,
}: SessionCreateDialogHostProps) {
  const profileDestination = useAggregateDestination();
  const sessionCreate = useSessionCreateDialogViewModel(
    { agents, activeWorkspace, scope, projectWorkspaceId, homeWorkspaceId },
    store
  );

  return (
    <SessionCreateDialog
      profileDestination={profileDestination}
      agents={sessionCreate.agents}
      destinationLabel={sessionCreate.destinationLabel}
      destinationReady={sessionCreate.destinationReady}
      environment={sessionCreate.environment}
      environmentListingError={sessionCreate.environmentListingError}
      environmentListingState={sessionCreate.environmentListingState}
      firstMessage={sessionCreate.firstMessage}
      isAwaitingEnvironment={sessionCreate.isAwaitingEnvironment}
      isSubmitting={sessionCreate.isSubmitting}
      mode={sessionCreate.mode}
      networkParticipation={sessionCreate.networkParticipation}
      onAgentChange={sessionCreate.onAgentChange}
      onCancelEnvironment={sessionCreate.onCancelEnvironment}
      onFirstMessageChange={sessionCreate.onFirstMessageChange}
      onModeChange={sessionCreate.onModeChange}
      onNetworkParticipationChange={sessionCreate.onNetworkParticipationChange}
      onOpenChange={sessionCreate.onOpenChange}
      onSessionNameChange={sessionCreate.onSessionNameChange}
      onSubmit={sessionCreate.submit}
      open={sessionCreate.open}
      restoreFocusOnClose={sessionCreate.restoreFocusOnClose}
      scope={sessionCreate.scope}
      selectedAgentName={sessionCreate.selectedAgentName}
      sessionName={sessionCreate.sessionName}
      sessionRoot={sessionCreate.sessionRoot}
      submitError={sessionCreate.submitError}
    />
  );
}
