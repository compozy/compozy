import { ArrowRightLeft } from "lucide-react";
import type { FormEvent } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  FieldError,
  Spinner,
} from "@compozy/ui";

import type { SessionDerivePlacementHandlers } from "../hooks/use-session-derive";
import { useSessionContinueDialog } from "../hooks/use-session-continue-dialog";
import { useSessionContinueRuntimeOptions } from "../hooks/use-session-continue-runtime-options";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import type { SessionPayload } from "../types";
import { SessionContinueFields } from "./session-continue-fields";
import { SessionDerivePlacementField } from "./session-derive-placement";
import { SessionDerivePreviewLine } from "./session-derive-preview";

export interface SessionContinueDialogProps {
  source: SessionPayload;
  /** The source's workspace: the child is created there. */
  workspaceId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  placement: SessionDerivePlacementHandlers;
}

/**
 * Continue with another agent: the Start-session shell with the source already
 * known. The context line is measured on open and gates the primary; the
 * source is never touched.
 */
export function SessionContinueDialog({
  source,
  workspaceId,
  open,
  onOpenChange,
  placement,
}: SessionContinueDialogProps) {
  const model = useSessionContinueDialog({
    source,
    workspaceId,
    open,
    onClose: () => onOpenChange(false),
    placement,
  });
  const runtimeOptions = useSessionContinueRuntimeOptions(workspaceId, open);

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
        data-testid="session-continue-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description="Start a new session for another agent with this conversation carried over. This session stays unchanged."
          eyebrow="Operate · Session"
          icon={ArrowRightLeft}
          onClose={model.isSubmitting ? undefined : () => handleOpenChange(false)}
          title="Continue with another agent"
        />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col gap-4">
            <SessionContinueFields model={model} runtimeOptions={runtimeOptions} />

            <SessionDerivePreviewLine view={model.previewView} />

            {model.canChoosePlacement ? (
              <SessionDerivePlacementField
                disabled={model.isSubmitting}
                onChange={model.onPlacementChange}
                value={model.placement}
              />
            ) : null}

            {model.submitError ? (
              <FieldError data-testid="session-continue-submit-error">
                {model.submitError}
              </FieldError>
            ) : null}

            {model.isSubmitting ? (
              <p
                aria-live="polite"
                className="flex items-center gap-2 text-form-hint text-subtle"
                data-testid="session-continue-pending-status"
                role="status"
              >
                <Spinner aria-hidden="true" className="size-3" />
                Starting the new session…
              </p>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            cancelDisabled={model.isSubmitting}
            hint={
              <span className="truncate" data-testid="session-continue-source-note">
                From <b className="font-medium text-fg">{getSessionDisplayTitle(source)}</b> ·{" "}
                {source.agent_name}
              </span>
            }
            isSaving={model.isSubmitting}
            onCancel={() => handleOpenChange(false)}
            primaryDisabled={!model.canSubmit}
            primaryLabel="Continue"
            primaryTestId="session-continue-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}
