import { GitFork } from "lucide-react";
import type { FormEvent } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Spinner,
} from "@compozy/ui";

import type { SessionForkPoint } from "../contexts/session-fork-context-value";
import type { SessionDerivePlacementHandlers } from "../hooks/use-session-derive";
import { useSessionForkDialog } from "../hooks/use-session-fork-dialog";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import type { SessionPayload } from "../types";
import { SessionDeriveSubmitOutcome } from "./session-derive-committed-child";
import { SessionDerivePreviewLine } from "./session-derive-preview";
import { SessionForkAgentField } from "./session-fork-agent-field";

export interface SessionForkDialogProps {
  source: SessionPayload;
  /** The source's workspace: the child is created there. */
  workspaceId: string;
  /** Fork from here; `null` forks the whole session. */
  point: SessionForkPoint | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The child replaces the source in this window; a host without one opens it in its own. */
  placement: SessionDerivePlacementHandlers;
}

/**
 * Fork session: the Continue shell with nothing to choose — the agent is locked,
 * the context line carries the daemon's own decisions (what the cut carries,
 * turn settled, native clone possible), and the child opens in this window.
 * The source is never touched.
 */
export function SessionForkDialog({
  source,
  workspaceId,
  point,
  open,
  onOpenChange,
  placement,
}: SessionForkDialogProps) {
  const model = useSessionForkDialog({
    source,
    workspaceId,
    open,
    point,
    onClose: () => onOpenChange(false),
    placement,
  });

  const handleOpenChange = (nextOpen: boolean) => {
    if (model.isSubmitting && !nextOpen) return;
    onOpenChange(nextOpen);
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    model.submit();
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        aria-busy={model.isSubmitting || undefined}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="session-fork-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Start a second session with the same agent and this conversation. This session stays unchanged."
          eyebrow="Operate · Session"
          icon={GitFork}
          onClose={model.isSubmitting ? undefined : () => handleOpenChange(false)}
          title="Fork session"
        />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col gap-4">
            <SessionForkAgentField agentName={model.agentName} provider={model.provider} />

            <SessionDerivePreviewLine
              nativeClone={model.nativeClone}
              refusal={model.previewRefusal}
              view={model.previewView}
            />

            <SessionDeriveSubmitOutcome
              error={model.submitError}
              model={model.committedChild}
              testIdPrefix="session-fork"
            />

            {model.isSubmitting ? (
              <p
                aria-live="polite"
                className="flex items-center gap-2 text-form-hint text-subtle"
                data-testid="session-fork-pending-status"
                role="status"
              >
                <Spinner aria-hidden="true" className="size-3" />
                Starting the new session…
              </p>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            cancelDisabled={model.isSubmitting}
            cancelTestId="session-fork-cancel"
            // After a fence conflict nothing here can succeed; the way on is to reopen.
            cancelLabel={model.fenceConflict ? "Close" : "Cancel"}
            hint={
              <span className="truncate" data-testid="session-fork-source-note">
                From <b className="font-medium text-fg">{getSessionDisplayTitle(source)}</b>
              </span>
            }
            isSaving={model.isSubmitting}
            onCancel={() => handleOpenChange(false)}
            primaryDisabled={!model.canSubmit}
            primaryLabel="Fork session"
            primaryTestId="session-fork-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}
