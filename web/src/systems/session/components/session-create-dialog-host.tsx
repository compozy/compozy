import {
  useSessionCreateDialogViewModel,
  type SessionCreateDialogController,
} from "../hooks/use-session-create-dialog";
import { SessionCreateDialog } from "./session-create-dialog";
import type { AgentPayload } from "@/systems/agent";
import type { DataScope, WorkspacePayload } from "@/systems/workspace";

interface SessionCreateDialogHostProps extends SessionCreateDialogController {
  agents: AgentPayload[] | undefined;
  activeWorkspace: WorkspacePayload | undefined;
  dataScope: DataScope | null;
  projectWorkspaceId: string | null;
  /** The focused window's binding: first in the All-projects project prefill. */
  focusedBinding?: string | null;
}

/** Isolates session-create selectors from the desktop chrome. */
export function SessionCreateDialogHost({
  agents,
  activeWorkspace,
  dataScope,
  projectWorkspaceId,
  focusedBinding,
  store,
}: SessionCreateDialogHostProps) {
  const sessionCreate = useSessionCreateDialogViewModel(
    { agents, activeWorkspace, dataScope, projectWorkspaceId, focusedBinding },
    store
  );

  return (
    <SessionCreateDialog
      destinationAxes={sessionCreate.destinationAxes}
      agents={sessionCreate.agents}
      destinationLabel={sessionCreate.destinationLabel}
      destinationReady={sessionCreate.destinationReady}
      environment={sessionCreate.environment}
      environmentListingError={sessionCreate.environmentListingError}
      environmentListingState={sessionCreate.environmentListingState}
      isAwaitingEnvironment={sessionCreate.isAwaitingEnvironment}
      isSubmitting={sessionCreate.isSubmitting}
      mode={sessionCreate.mode}
      onAgentChange={sessionCreate.onAgentChange}
      onCancelEnvironment={sessionCreate.onCancelEnvironment}
      onModeChange={sessionCreate.onModeChange}
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
