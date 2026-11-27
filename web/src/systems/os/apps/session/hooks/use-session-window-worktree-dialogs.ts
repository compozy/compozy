import { use } from "react";

import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import type { WorktreePayload } from "@/systems/workspace";

import { WorktreeDialogActionsContext } from "../../../contexts/worktree-dialog-actions-context";

export function useSessionWindowWorktreeDialogs() {
  const computerId = useCurrentComputerId();
  const dialogs = use(WorktreeDialogActionsContext);
  return {
    computerId,
    onOpenWorktreeContext: dialogs
      ? (workspaceId: string, worktree: WorktreePayload) =>
          dialogs.requestContextWorktree(workspaceId, worktree, computerId)
      : undefined,
    onResolveMissingWorktree: dialogs
      ? (workspaceId: string, worktree: WorktreePayload) =>
          dialogs.requestResolveMissingWorktree(workspaceId, worktree, computerId)
      : undefined,
  };
}
