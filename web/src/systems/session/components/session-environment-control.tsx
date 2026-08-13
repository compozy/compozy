import type { Ref } from "react";

import type { SessionWorktreeBinding } from "../hooks/use-session-worktree-binding";
import { useSessionEnvironmentControl } from "../hooks/use-session-environment-control";
import { SessionEnvironmentChip } from "./session-environment-chip";
import { SessionWorktreeForkDialog } from "./session-worktree-fork-dialog";

interface SessionEnvironmentControlProps {
  binding: SessionWorktreeBinding;
  workspaceId: string;
  workspaceName: string;
  sessionId: string;
  sessionTitle: string;
}

export interface SessionEnvironmentControlHandle {
  openFork: () => void;
}

/**
 * The composer's environment control.
 *
 * A session's worktree binding cannot change, so this states the binding and,
 * when the daemon allows it, opens the fork dialog. It never offers an in-place
 * switch, because none exists.
 */
export function SessionEnvironmentControl({
  binding,
  workspaceId,
  workspaceName,
  sessionId,
  sessionTitle,
  ref,
}: SessionEnvironmentControlProps & { ref?: Ref<SessionEnvironmentControlHandle> }) {
  const control = useSessionEnvironmentControl({ ref, sessionId, workspaceId });

  const label = binding.bound ? (binding.worktree?.name ?? binding.worktreeId) : workspaceName;

  return (
    <>
      <SessionEnvironmentChip
        forkUnavailableReason={binding.forkUnavailableReason}
        label={label}
        onFork={binding.forkAvailable ? control.open : undefined}
        state={binding.bound ? "worktree" : "root"}
      />
      {control.forkOpen ? (
        <SessionWorktreeForkDialog
          blockedReason={binding.forkUnavailableReason}
          currentWorktreeId={binding.bound ? binding.worktreeId : undefined}
          isPending={control.isPending}
          onConfirm={control.confirm}
          onNewWorktree={() => control.materialization.start("")}
          onOpenChange={control.setOpen}
          onTargetChange={control.setTarget}
          open
          sessionTitle={sessionTitle}
          targetWorktreeId={control.target}
          worktrees={binding.worktrees}
          materialization={control.materialization}
        />
      ) : null}
    </>
  );
}
