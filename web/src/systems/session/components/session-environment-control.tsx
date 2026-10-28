import type { Ref } from "react";
import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from "@compozy/ui";
import { SessionWorktreeBindingDialog } from "./session-worktree-binding-dialog";

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

/** Changes the environment in place or opens the existing worktree fork flow. */
export function SessionEnvironmentControl({
  binding,
  workspaceId,
  workspaceName,
  sessionId,
  sessionTitle,
  ref,
}: SessionEnvironmentControlProps & { ref?: Ref<SessionEnvironmentControlHandle> }) {
  const control = useSessionEnvironmentControl({
    ref,
    sessionId,
    workspaceId,
    currentWorktreeId: binding.worktreeId,
  });

  const label = binding.bound ? (binding.worktree?.name ?? binding.worktreeId) : workspaceName;

  return (
    <>
      <SessionEnvironmentChip
        menu={
          <>
            <DropdownMenuItem
              disabled={!binding.bindingAvailable || control.bindingPending}
              onClick={control.openBinding}
            >
              {binding.bound ? "Change worktree…" : "Use worktree…"}
            </DropdownMenuItem>
            {binding.bound ? (
              <DropdownMenuItem
                disabled={!binding.bindingAvailable || control.bindingPending}
                onClick={control.selectWorkspaceRoot}
              >
                Use workspace root
              </DropdownMenuItem>
            ) : null}
            {binding.bindingUnavailableReason ? (
              <DropdownMenuGroup>
                <DropdownMenuLabel>{binding.bindingUnavailableReason}</DropdownMenuLabel>
              </DropdownMenuGroup>
            ) : null}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={!binding.forkAvailable}
              onClick={control.open}
              title={binding.forkUnavailableReason}
            >
              Fork into a worktree…
            </DropdownMenuItem>
          </>
        }
        label={label}
        onFork={binding.forkAvailable ? control.open : undefined}
        state={binding.bound ? "worktree" : "root"}
      />
      {control.bindingOpen ? (
        <SessionWorktreeBindingDialog
          available={binding.bindingAvailable === true}
          blockedReason={binding.bindingUnavailableReason}
          currentWorktreeId={binding.worktreeId}
          isPending={control.bindingPending}
          materialization={control.materialization}
          onConfirm={control.confirmBinding}
          onNewWorktree={control.startNewWorktree}
          onOpenChange={control.setOpen}
          onTargetChange={control.setTarget}
          target={control.target}
          worktrees={binding.worktrees}
        />
      ) : null}
      {control.forkOpen ? (
        <SessionWorktreeForkDialog
          blockedReason={binding.forkUnavailableReason}
          currentWorktreeId={binding.bound ? binding.worktreeId : undefined}
          isPending={control.isPending}
          onConfirm={control.confirm}
          onNewWorktree={control.startNewWorktree}
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
