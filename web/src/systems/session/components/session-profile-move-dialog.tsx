import { UserRound } from "@compozy/ui/icons";
import type { FormEvent } from "react";

import {
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  Spinner,
  toast,
} from "@compozy/ui";

import type { SessionProfileMoveTarget } from "../contexts/session-profile-move-context-value";
import { useMoveSessionToProfile } from "../hooks/use-move-session-to-profile";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionProfileMoveRefusal } from "../lib/session-profile-move";
import type { SessionPayload } from "../types";

export interface SessionProfileMoveDialogProps {
  source: SessionPayload;
  target: SessionProfileMoveTarget;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Move to profile: a copy of the session, with its conversation, starts in the
 * target profile and this one is archived. A session's profile never changes,
 * so the copy is a new session. Refusals the row can see show before submit;
 * the daemon's own (uncommitted changes in the worktree) show after it.
 */
export function SessionProfileMoveDialog({
  source,
  target,
  open,
  onOpenChange,
}: SessionProfileMoveDialogProps) {
  const move = useMoveSessionToProfile();
  const refusal = sessionProfileMoveRefusal(source);
  const title = getSessionDisplayTitle(source);
  const submitError = move.error instanceof Error ? move.error.message : null;

  const handleOpenChange = (nextOpen: boolean) => {
    if (move.isPending && !nextOpen) return;
    onOpenChange(nextOpen);
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (refusal !== null || move.isPending) return;
    move.mutate(
      {
        id: source.id,
        workspace_id: source.workspace_id,
        profile_name: source.profile_name,
        targetProfile: target.name,
      },
      {
        onSuccess: () => {
          toast.success(`Moved to ${target.name}`, {
            description: "The original is archived here.",
          });
          onOpenChange(false);
        },
      }
    );
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        aria-busy={move.isPending || undefined}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="session-profile-move-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`A copy with this conversation starts in ${target.name}. This session is archived here and keeps its history.`}
          eyebrow="Operate · Session"
          icon={UserRound}
          onClose={move.isPending ? undefined : () => handleOpenChange(false)}
          title="Move to profile"
        />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col gap-3 text-form-hint text-subtle">
            <p data-testid="session-profile-move-usage-note">
              Usage so far stays counted in {source.profile_name}.
            </p>
            {source.worktree_id ? (
              <p data-testid="session-profile-move-worktree-note">
                Its worktree moves too: the branch is checked out again for {target.name}.
              </p>
            ) : null}
            {refusal !== null ? (
              <p
                className="text-destructive"
                data-testid="session-profile-move-refusal"
                role="alert"
              >
                {refusal}
              </p>
            ) : null}
            {submitError !== null ? (
              <p className="text-destructive" data-testid="session-profile-move-error" role="alert">
                {submitError}
              </p>
            ) : null}
            {move.isPending ? (
              <p
                aria-live="polite"
                className="flex items-center gap-2"
                data-testid="session-profile-move-pending-status"
                role="status"
              >
                <Spinner aria-hidden="true" className="size-3" />
                Moving the session…
              </p>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            cancelDisabled={move.isPending}
            cancelTestId="session-profile-move-cancel"
            hint={
              <span className="truncate" data-testid="session-profile-move-source-note">
                <b className="font-medium text-fg">{title}</b>
              </span>
            }
            isSaving={move.isPending}
            onCancel={() => handleOpenChange(false)}
            primaryDisabled={refusal !== null || move.isPending}
            primaryLabel={`Move to ${target.name}`}
            primaryTestId="session-profile-move-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}
