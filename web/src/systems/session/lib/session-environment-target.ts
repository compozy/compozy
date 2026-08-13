import type { WorktreePayload } from "@/systems/workspace";

/**
 * Where a session about to be created will run.
 *
 * `new` is an intent, not a binding: the worktree is materialized through the
 * worktree API first and the session is only created once a real id exists.
 */
export type SessionEnvironmentBoundTarget =
  | { kind: "root" }
  | { kind: "worktree"; worktreeId: string };

export type SessionEnvironmentTarget =
  | SessionEnvironmentBoundTarget
  | {
      kind: "new";
      name: string;
      /** Target to restore when the pending materialization is canceled. */
      previous?: SessionEnvironmentBoundTarget;
    };

export const ROOT_ENVIRONMENT_TARGET: SessionEnvironmentTarget = { kind: "root" };

/** Locked mode vocabulary. Never "None", "Unset", "Root", or "Inherit workspace". */
export const WORKSPACE_ROOT_LABEL = "Workspace root";
export const NEW_WORKTREE_LABEL = "New worktree…";

/** Only a ready worktree can host a session; pending and missing rows are not offers. */
export function selectableWorktrees(
  worktrees: readonly WorktreePayload[] | undefined
): WorktreePayload[] {
  return (worktrees ?? []).filter(worktree => worktree.state === "ready");
}

export function findWorktree(
  worktrees: readonly WorktreePayload[] | undefined,
  worktreeId: string
): WorktreePayload | undefined {
  return worktrees?.find(worktree => worktree.id === worktreeId);
}

/**
 * The label for a target. A `worktree` target whose record is gone resolves to
 * `undefined` so callers render the missing-selection error instead of echoing
 * an id the operator never typed.
 */
export function environmentTargetLabel(
  target: SessionEnvironmentTarget,
  worktrees: readonly WorktreePayload[] | undefined
): string | undefined {
  if (target.kind === "root") return WORKSPACE_ROOT_LABEL;
  if (target.kind === "new") return target.name.trim() === "" ? NEW_WORKTREE_LABEL : target.name;
  return findWorktree(worktrees, target.worktreeId)?.name;
}

/**
 * True when the target names a worktree that is no longer selectable — removed,
 * or no longer ready — which must block submission rather than silently fall back.
 */
export function isEnvironmentTargetMissing(
  target: SessionEnvironmentTarget,
  worktrees: readonly WorktreePayload[] | undefined
): boolean {
  if (target.kind !== "worktree") return false;
  const found = findWorktree(worktrees, target.worktreeId);
  return found === undefined || found.state !== "ready";
}

/** The session-create body fragment. `new` never reaches the request as an inline field. */
export function environmentCreateFields(
  target: SessionEnvironmentTarget,
  materializedWorktreeId?: string
): { worktree?: string } {
  if (target.kind === "worktree") return { worktree: target.worktreeId };
  if (target.kind === "new" && materializedWorktreeId) return { worktree: materializedWorktreeId };
  return {};
}
