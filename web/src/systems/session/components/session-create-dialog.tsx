import { Play } from "lucide-react";
import { type FormEvent, useLayoutEffect, useRef } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  FieldError,
  type EntityMode,
} from "@compozy/ui";

import {
  isNetworkParticipationDraftValid,
  type NetworkParticipationDraft,
} from "@/lib/network-participation";

import { SessionCreateAdvancedSection } from "./session-create-advanced-section";
import { SessionCreateFirstMessage } from "./session-create-first-message";
import { SessionEnvironmentField } from "./session-environment-field";
import { SessionCreateSimpleSection } from "./session-create-simple-section";
import type { AgentPayload } from "@/systems/agent";
import { WorkspaceScopeStatement, type WorkspaceScopeMode } from "@/systems/workspace";

export interface SessionCreateDialogProps {
  open: boolean;
  restoreFocusOnClose: boolean;
  onOpenChange: (open: boolean) => void;
  mode: EntityMode;
  onModeChange: (mode: EntityMode) => void;
  agents: AgentPayload[];
  scope: WorkspaceScopeMode;
  destinationLabel: string;
  sessionRoot: string;
  destinationReady: boolean;
  sessionName: string;
  onSessionNameChange: (next: string) => void;
  firstMessage: string;
  onFirstMessageChange: (next: string) => void;
  selectedAgentName: string;
  networkParticipation: NetworkParticipationDraft;
  onAgentChange: (agentName: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  /** Absent when the selected workspace is not git-backed. */
  environment?: React.ComponentProps<typeof SessionEnvironmentField>;
  onSubmit: () => void;
  isSubmitting: boolean;
  /** A submit is waiting on its worktree; Cancel stops that, not the dialog. */
  isAwaitingEnvironment: boolean;
  onCancelEnvironment: () => void;
  submitError: string | null;
}

function SessionCreateDialog({
  open,
  restoreFocusOnClose,
  onOpenChange,
  mode,
  onModeChange,
  agents,
  scope,
  destinationLabel,
  sessionRoot,
  destinationReady,
  sessionName,
  onSessionNameChange,
  firstMessage,
  onFirstMessageChange,
  selectedAgentName,
  networkParticipation,
  onAgentChange,
  onNetworkParticipationChange,
  environment,
  onSubmit,
  isSubmitting,
  isAwaitingEnvironment,
  onCancelEnvironment,
  submitError,
}: SessionCreateDialogProps) {
  const restoreFocusOnCloseRef = useRef(restoreFocusOnClose);
  useLayoutEffect(() => {
    restoreFocusOnCloseRef.current = restoreFocusOnClose;
  }, [restoreFocusOnClose]);

  const trimmedSelectedAgentName = selectedAgentName.trim();
  const hasAgents = agents.length > 0;
  const hasSelectedAgent = agents.some(agent => agent.name === trimmedSelectedAgentName);
  const canSubmit =
    !isSubmitting &&
    destinationReady &&
    !isAwaitingEnvironment &&
    hasAgents &&
    hasSelectedAgent &&
    isNetworkParticipationDraftValid(networkParticipation, ["named"]);

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmit) return;
    onSubmit();
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (isSubmitting && !nextOpen) return;
    onOpenChange(nextOpen);
  };

  // While a worktree is being created, Cancel is about that creation — the
  // operator asked to stop making an environment, not to discard what they wrote.
  const handleCancel = () => {
    if (isAwaitingEnvironment) {
      onCancelEnvironment();
      return;
    }
    handleOpenChange(false);
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="session-create-dialog"
        finalFocus={isSubmitting ? false : () => restoreFocusOnCloseRef.current}
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          eyebrow="Operate · Session"
          icon={Play}
          onClose={isSubmitting ? undefined : () => handleOpenChange(false)}
          title="Start session"
        />

        <EntityModeToolbar mode={mode} onModeChange={onModeChange} testIdPrefix="session-create" />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col gap-4">
            <SessionCreateSimpleSection
              agents={agents}
              isSubmitting={isSubmitting}
              onAgentChange={onAgentChange}
              selectedAgentName={selectedAgentName}
              workspaceSelected={destinationReady}
            />

            <SessionCreateFirstMessage
              disabled={isSubmitting || isAwaitingEnvironment}
              onChange={onFirstMessageChange}
              value={firstMessage}
            />

            {mode === "advanced" ? (
              <SessionCreateAdvancedSection
                environment={environment}
                isSubmitting={isSubmitting || isAwaitingEnvironment}
                networkParticipation={networkParticipation}
                onNetworkParticipationChange={onNetworkParticipationChange}
                onSessionNameChange={onSessionNameChange}
                sessionName={sessionName}
              />
            ) : null}

            {submitError ? (
              <FieldError
                className="mt-4 text-form-hint text-danger"
                data-testid="session-create-submit-error"
              >
                {submitError}
              </FieldError>
            ) : null}

            {isAwaitingEnvironment ? (
              <p
                aria-live="polite"
                className="mt-4 text-form-hint text-subtle"
                data-testid="session-create-environment-status"
                role="status"
              >
                Creating the environment. The session starts once it is ready.
              </p>
            ) : null}

            {isSubmitting ? (
              <p
                aria-live="polite"
                className="mt-4 text-form-hint text-subtle"
                data-testid="session-create-pending-status"
                role="status"
              >
                Starting the session. It opens as soon as CompozyOS durably accepts it.
              </p>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            hint={
              <WorkspaceScopeStatement
                destination={destinationLabel}
                kind="session"
                root={sessionRoot}
                scope={scope}
                variant="note"
              />
            }
            isSaving={isSubmitting}
            onCancel={handleCancel}
            primaryDisabled={!canSubmit}
            primaryLabel="Start session"
            primaryTestId="session-create-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}

export { SessionCreateDialog };
