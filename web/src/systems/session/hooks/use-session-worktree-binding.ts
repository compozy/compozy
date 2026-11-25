import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import type { ComputerId } from "@/lib/computer-scope";
import { useWorktrees, type WorktreePayload } from "@/systems/workspace";

import type { SessionPayload } from "../types";

import { findSessionCommand, type SessionCommandMenuCatalog } from "./use-session-commands";

export const WORKTREE_COMMAND_TOKEN = "/worktree";

interface UseSessionWorktreeBindingParams {
  workspaceId: string;
  worktreeId: string | undefined;
  computerId?: ComputerId;
  commandCatalog?: SessionCommandMenuCatalog;
  availability?: SessionPayload["worktree_binding"];
  enabled?: boolean;
}

export interface SessionWorktreeBinding {
  bindingAvailable?: boolean;
  bindingUnavailableReason?: string;
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

/** Resolves the current environment and daemon-reported mutation availability. */
export function useSessionWorktreeBinding({
  workspaceId,
  worktreeId,
  computerId,
  commandCatalog,
  availability,
  enabled = true,
}: UseSessionWorktreeBindingParams): SessionWorktreeBinding {
  const scopedComputerId = useCurrentComputerId(computerId);
  const boundId = worktreeId?.trim() ?? "";
  const worktrees = useWorktrees(workspaceId || null, {
    enabled: enabled && workspaceId !== "",
    computerId: scopedComputerId,
  });
  const command = findSessionCommand(commandCatalog, WORKTREE_COMMAND_TOKEN);
  const listing = worktrees.data?.worktrees ?? [];

  return {
    bindingAvailable: availability?.available === true,
    bindingUnavailableReason: availability?.unavailable_reason,
    worktreeId: boundId,
    worktree: boundId === "" ? undefined : listing.find(entry => entry.id === boundId),
    bound: boundId !== "",
    worktrees: listing,
    forkAvailable: command?.available === true,
    forkUnavailableReason: command?.available === false ? command.unavailableReason : undefined,
  };
}
