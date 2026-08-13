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
  selectedAgentName: string;
  networkParticipation: NetworkParticipationDraft;
  onAgentChange: (agentName: string) => void;
  onNetworkParticipationChange: (next: NetworkParticipationDraft) => void;
  onSubmit: () => void;
  isSubmitting: boolean;
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
  selectedAgentName,
  networkParticipation,
  onAgentChange,
  onNetworkParticipationChange,
  onSubmit,
  isSubmitting,
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
    hasAgents &&
    hasSelectedAgent &&
    isNetworkParticipationDraftValid(networkParticipation, ["named"]);

  const submitIfAllowed = () => {
    if (!canSubmit) return;
    onSubmit();
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    submitIfAllowed();
  };

  const handleOpenChange = (nextOpen: boolean) => {
    if (isSubmitting && !nextOpen) return;
    onOpenChange(nextOpen);
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

            {mode === "advanced" ? (
              <SessionCreateAdvancedSection
                isSubmitting={isSubmitting}
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
            onCancel={() => handleOpenChange(false)}
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
