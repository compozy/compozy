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
} from "@compozy/ui";

export interface SessionProfileMoveDialogViewProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The session's display title. */
  title: string;
  /** The profile the copy starts in. */
  targetName: string;
  /** The session's own profile, where its usage so far stays counted. */
  profileName: string;
  /** The session runs in a worktree, which moves with it. */
  hasWorktree: boolean;
  /** Why the move cannot be tried, as far as the row knows; null when it can. */
  refusal: string | null;
  /** The daemon's refusal or failure after submit. */
  error: string | null;
  pending: boolean;
  onSubmit: () => void;
}

/**
 * The Move to profile dialog: what moves where, why it cannot (before or after
 * submit), and the confirm. Data and the move itself come from its container.
 */
export function SessionProfileMoveDialogView({
  open,
  onOpenChange,
  title,
  targetName,
  profileName,
  hasWorktree,
  refusal,
  error,
  pending,
  onSubmit,
}: SessionProfileMoveDialogViewProps) {
  const handleOpenChange = (nextOpen: boolean) => {
    if (pending && !nextOpen) return;
    onOpenChange(nextOpen);
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (refusal !== null || pending) return;
    onSubmit();
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        aria-busy={pending || undefined}
        className={`grid-rows-[auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("sm")}`}
        data-testid="session-profile-move-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={`A copy with this conversation starts in ${targetName}. This session is archived here and keeps its history.`}
          eyebrow="Operate · Session"
          icon={UserRound}
          onClose={pending ? undefined : () => handleOpenChange(false)}
          title="Move to profile"
        />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col gap-3 text-form-hint text-subtle">
            <p data-testid="session-profile-move-usage-note">
              Usage so far stays counted in {profileName}.
            </p>
            {hasWorktree ? (
              <p data-testid="session-profile-move-worktree-note">
                Its worktree moves too: the branch is checked out again for {targetName}.
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
            {error !== null ? (
              <p className="text-destructive" data-testid="session-profile-move-error" role="alert">
                {error}
              </p>
            ) : null}
            {pending ? (
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
            cancelDisabled={pending}
            cancelTestId="session-profile-move-cancel"
            hint={
              <span className="truncate" data-testid="session-profile-move-source-note">
                <b className="font-medium text-fg">{title}</b>
              </span>
            }
            isSaving={pending}
            onCancel={() => handleOpenChange(false)}
            primaryDisabled={refusal !== null || pending}
            primaryLabel={`Move to ${targetName}`}
            primaryTestId="session-profile-move-submit"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}
