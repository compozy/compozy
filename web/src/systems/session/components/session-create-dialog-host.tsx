import { Field, FieldLabel } from "@compozy/ui";

import { RunOnNotices, RunOnPicker } from "@/systems/computers";
import type { AgentPayload } from "@/systems/agent";
import type { DataScope, WorkspacePayload } from "@/systems/workspace";

import {
  useSessionCreateDialogViewModel,
  type SessionCreateDialogController,
} from "../hooks/use-session-create-dialog";
import { SessionCreateDialog } from "./session-create-dialog";

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
  const showRunOn =
    sessionCreate.computers.some(computer => computer.enabled) ||
    !sessionCreate.localComputer.localRunAgents;

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
      notice={
        <RunOnNotices
          autoCandidates={sessionCreate.autoCandidates}
          autoNoEligible={sessionCreate.autoNoEligible}
          autoNoEligibleReason={sessionCreate.autoNoEligibleReason}
          isReconnectingOffline={
            sessionCreate.offlineComputer !== null &&
            sessionCreate.reconnectingComputerIds.includes(sessionCreate.offlineComputer.id)
          }
          localExecutionBlocked={sessionCreate.localExecutionBlocked}
          offlineComputer={sessionCreate.offlineComputer}
          onReconnectOffline={sessionCreate.onReconnectComputer}
          projectName={sessionCreate.destinationLabel}
        />
      }
      onAgentChange={sessionCreate.onAgentChange}
      onCancelEnvironment={sessionCreate.onCancelEnvironment}
      onModeChange={sessionCreate.onModeChange}
      onOpenChange={sessionCreate.onOpenChange}
      onSessionNameChange={sessionCreate.onSessionNameChange}
      onSubmit={sessionCreate.submit}
      open={sessionCreate.open}
      restoreFocusOnClose={sessionCreate.restoreFocusOnClose}
      runOn={
        showRunOn ? (
          <Field>
            <FieldLabel htmlFor="session-create-run-on">Run on</FieldLabel>
            <RunOnPicker
              allowAuto
              autoLoading={sessionCreate.autoLoading}
              autoResolution={sessionCreate.autoResolution}
              computers={sessionCreate.computers}
              disabled={sessionCreate.isSubmitting}
              hideWhenSingleComputer
              loadBalancingEnabled={sessionCreate.loadBalancingEnabled}
              localComputer={sessionCreate.localComputer}
              locked={sessionCreate.computerLocked}
              onAddComputer={sessionCreate.onAddComputer}
              onChange={sessionCreate.onComputerChange}
              onReconnect={sessionCreate.onReconnectComputer}
              onUpgrade={sessionCreate.onUpgradeComputer}
              reconnectingIds={sessionCreate.reconnectingComputerIds}
              testId="session-create-run-on"
              triggerId="session-create-run-on"
              value={sessionCreate.computerId}
            />
          </Field>
        ) : undefined
      }
      scope={sessionCreate.scope}
      selectedAgentName={sessionCreate.selectedAgentName}
      sessionName={sessionCreate.sessionName}
      sessionRoot={sessionCreate.sessionRoot}
      submitDisabled={sessionCreate.submitDisabled}
      submitError={sessionCreate.submitError}
    />
  );
}
