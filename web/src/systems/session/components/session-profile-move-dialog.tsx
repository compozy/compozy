import { toast } from "@compozy/ui";

import type { SessionProfileMoveTarget } from "../contexts/session-profile-move-context-value";
import { useMoveSessionToProfile } from "../hooks/use-move-session-to-profile";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import { sessionProfileMoveRefusal } from "../lib/session-profile-move";
import type { SessionPayload } from "../types";
import { SessionProfileMoveDialogView } from "./session-profile-move-dialog-view";

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
  const submit = () =>
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

  return (
    <SessionProfileMoveDialogView
      error={move.error instanceof Error ? move.error.message : null}
      hasWorktree={Boolean(source.worktree_id)}
      onOpenChange={onOpenChange}
      onSubmit={submit}
      open={open}
      pending={move.isPending}
      profileName={source.profile_name}
      refusal={sessionProfileMoveRefusal(source)}
      targetName={target.name}
      title={getSessionDisplayTitle(source)}
    />
  );
}
