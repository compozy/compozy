import { useWorktrees, type WorktreePayload } from "@/systems/workspace";

import { findSessionCommand, type SessionCommandMenuCatalog } from "./use-session-commands";

export const WORKTREE_COMMAND_TOKEN = "/worktree";

interface UseSessionWorktreeBindingParams {
  workspaceId: string;
  worktreeId: string | undefined;
  commandCatalog?: SessionCommandMenuCatalog;
  enabled?: boolean;
}

export interface SessionWorktreeBinding {
  worktreeId: string;
  /** Undefined when the bound record no longer resolves — the binding is missing. */
  worktree: WorktreePayload | undefined;
  bound: boolean;
  /** Every ready worktree in the session's workspace, for the fork target list. */
  worktrees: readonly WorktreePayload[];
  /** True only when the daemon reports `/worktree` as runnable right now. */
  forkAvailable: boolean;
  /** The daemon's own reason when it is not. Rendered verbatim, never reworded. */
  forkUnavailableReason: string | undefined;
}

/**
 * What a live session's environment actually is.
 *
 * The binding is immutable once a session exists, so this hook only reads: it
 * resolves the bound record and asks the command catalog whether forking is
 * possible. Fork availability is never computed locally — the daemon gates it
 * on session state and supplies the reason when it refuses.
 */
export function useSessionWorktreeBinding({
  workspaceId,
  worktreeId,
  commandCatalog,
  enabled = true,
}: UseSessionWorktreeBindingParams): SessionWorktreeBinding {
  const boundId = worktreeId?.trim() ?? "";
  const worktrees = useWorktrees(workspaceId || null, {
    enabled: enabled && workspaceId !== "",
  });
  const command = findSessionCommand(commandCatalog, WORKTREE_COMMAND_TOKEN);
  const listing = worktrees.data?.worktrees ?? [];

  return {
    worktreeId: boundId,
    worktree: boundId === "" ? undefined : listing.find(entry => entry.id === boundId),
    bound: boundId !== "",
    worktrees: listing,
    forkAvailable: command?.available === true,
    forkUnavailableReason: command?.available === false ? command.unavailableReason : undefined,
  };
}
