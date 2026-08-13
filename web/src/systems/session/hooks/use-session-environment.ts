import {
  useWorktreeMaterialization,
  useWorktrees,
  type WorktreeMaterialization,
  type WorktreeMaterializationStatus,
  type WorktreePayload,
} from "@/systems/workspace";

import {
  isEnvironmentTargetMissing,
  ROOT_ENVIRONMENT_TARGET,
  type SessionEnvironmentTarget,
} from "../lib/session-environment-target";

interface UseSessionEnvironmentParams {
  workspaceId: string;
  target: SessionEnvironmentTarget;
  onTargetChange: (next: SessionEnvironmentTarget) => void;
  enabled?: boolean;
}

/** What a submit can do with the chosen environment right now. */
export type SessionEnvironmentReadiness = "ready" | "materializing" | "blocked";

export interface SessionEnvironmentModel {
  /** Undefined when the workspace is not git-backed — the control stays absent. */
  field:
    | {
        value: SessionEnvironmentTarget;
        worktrees: readonly WorktreePayload[];
        materialization: WorktreeMaterialization;
        onChange: (next: SessionEnvironmentTarget) => void;
      }
    | undefined;
  /** The id to bind the session to, once one genuinely exists. */
  worktreeId: string | undefined;
  status: WorktreeMaterializationStatus;
  /** Starts creation if the target needs it, and reports what the submit may do. */
  ensureReady: () => SessionEnvironmentReadiness;
  /** Abandons an in-flight creation and restores the supplied prior target. */
  cancelPending: (restoreTarget?: SessionEnvironmentTarget) => void;
}

/**
 * Resolves the session-create environment choice into a real binding.
 *
 * Nothing is created while the operator is still deciding: picking "New
 * worktree…" only records the intent, and the checkout is made when they submit
 * the first message. A dialog that is abandoned therefore leaves no worktree
 * behind, and a session is still only ever created against one that exists.
 */
export function useSessionEnvironment({
  workspaceId,
  target,
  onTargetChange,
  enabled = true,
}: UseSessionEnvironmentParams): SessionEnvironmentModel {
  const worktrees = useWorktrees(workspaceId || null, {
    enabled: enabled && workspaceId !== "",
  });
  const materialization = useWorktreeMaterialization(workspaceId);
  const listing = worktrees.data;

  const cancelPending = (restoreTarget = ROOT_ENVIRONMENT_TARGET) => {
    if (materialization.status === "idle") return;
    // `cancel` rolls the daemon's artifacts back once a row exists. When the 202
    // has not arrived yet, the materialization store remembers this intent and
    // cancels the row as soon as its id becomes available.
    materialization.cancel();
    materialization.reset();
    onTargetChange(restoreTarget);
  };

  if (!listing?.repo.git_backed) {
    return {
      field: undefined,
      worktreeId: undefined,
      status: "idle",
      ensureReady: () => "ready",
      cancelPending: () => {},
    };
  }

  const handleChange = (next: SessionEnvironmentTarget) => {
    if (next.kind !== "new") materialization.reset();
    onTargetChange(next);
  };

  const materialized = target.kind === "new" ? materialization.worktree : undefined;
  const worktreeId =
    target.kind === "worktree"
      ? target.worktreeId
      : materialization.status === "ready"
        ? materialized?.id
        : undefined;

  const ensureReady = (): SessionEnvironmentReadiness => {
    if (isEnvironmentTargetMissing(target, listing.worktrees)) return "blocked";
    if (target.kind !== "new") return "ready";
    if (materialization.status === "ready") return "ready";
    if (materialization.status === "creating" || materialization.status === "pending") {
      return "materializing";
    }
    materialization.start(target.name);
    return "materializing";
  };

  return {
    field: {
      value: target,
      worktrees: listing.worktrees,
      materialization,
      onChange: handleChange,
    },
    worktreeId,
    status: materialization.status,
    ensureReady,
    cancelPending,
  };
}
