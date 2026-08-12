import {
  useSessionCreateDialogViewModel,
  type SessionCreateDialogController,
} from "../hooks/use-session-create-dialog";
import { SessionCreateDialog } from "./session-create-dialog";
import type { AgentPayload } from "@/systems/agent";
import type { WorkspacePayload } from "@/systems/workspace";

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
      isSubmitting={sessionCreate.isSubmitting}
      mode={sessionCreate.mode}
      networkParticipation={sessionCreate.networkParticipation}
      onAgentChange={sessionCreate.onAgentChange}
      onModeChange={sessionCreate.onModeChange}
      onNetworkParticipationChange={sessionCreate.onNetworkParticipationChange}
      onOpenChange={sessionCreate.onOpenChange}
      onSessionNameChange={sessionCreate.onSessionNameChange}
      onSubmit={sessionCreate.submit}
      onWorkspaceChange={sessionCreate.onWorkspaceChange}
      open={sessionCreate.open}
      restoreFocusOnClose={sessionCreate.restoreFocusOnClose}
      selectedAgentName={sessionCreate.selectedAgentName}
      sessionName={sessionCreate.sessionName}
      submitError={sessionCreate.submitError}
      userHomeDir={sessionCreate.userHomeDir}
      workspace={sessionCreate.workspace}
      workspaceId={sessionCreate.workspaceId}
      workspaces={sessionCreate.workspaces}
    />
  );
}
