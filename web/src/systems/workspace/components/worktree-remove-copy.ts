import {
  WORKTREE_ERROR_CODES,
  type WorktreeRefusal,
  type WorktreeRemovalRisk,
} from "../lib/worktree-refusal";

/**
 * Which removal state the dialog is in. Driven entirely by the daemon's decoded
 * refusal plus the explicit force escalation — never by client guesswork about
 * whether a tree looks dirty.
 */
export type WorktreeRemoveStage =
  | "clean"
  | "bound-idle"
  | "bound-running"
  | "dirty-refusal"
  | "force"
  | "downgrade";

export function resolveRemoveStage(
  refusal: WorktreeRefusal | null,
  forceRequested: boolean,
  boundSessions: { running: number; idle: number }
): WorktreeRemoveStage {
  if (forceRequested) return "force";
  if (refusal?.code === WORKTREE_ERROR_CODES.sessionActive) return "bound-running";
  if (
    refusal?.code === WORKTREE_ERROR_CODES.dirtyRequiresForce ||
    refusal?.code === WORKTREE_ERROR_CODES.unpushedRequiresForce
  ) {
    // A branch that also exists on a remote downgrades to an informational
    // note: the commits are recoverable, so a single confirm is honest. The
    // daemon may signal this on the refusal or inside the risk block.
    return refusal.downgrade || refusal.risk?.existsOnRemote ? "downgrade" : "dirty-refusal";
  }
  if (boundSessions.running > 0) return "bound-running";
  if (boundSessions.idle > 0) return "bound-idle";
  return "clean";
}

/** Titles quote the record label — never the branch, which is not deleted. */
export function removeDialogTitle(stage: WorktreeRemoveStage, worktreeName: string): string {
  switch (stage) {
    case "bound-running":
      return `A session is mid-turn in ${worktreeName}`;
    case "dirty-refusal":
      return `${worktreeName} holds work that exists nowhere else`;
    default:
      return `Remove “${worktreeName}” from disk? This cannot be undone.`;
  }
}

export function removeDialogDescription(
  stage: WorktreeRemoveStage,
  branch: string,
  risk: WorktreeRemovalRisk | null
): string {
  switch (stage) {
    case "bound-running":
      return "Nothing was removed. Removal waits until the turn ends or the session is stopped.";
    case "dirty-refusal":
      return "Nothing was removed. Commit & push the work out first, or force remove and lose it.";
    case "force":
      return risk
        ? `${risk.changedFiles} changed ${risk.changedFiles === 1 ? "file" : "files"} (+${risk.insertions} −${risk.deletions}) and ${risk.unpushedCommits} unpushed ${risk.unpushedCommits === 1 ? "commit" : "commits"} are deleted permanently. They exist nowhere else.`
        : "This deletes work that was never pushed anywhere.";
    default:
      return `The worktree directory is deleted. The branch ${branch} is not deleted.`;
  }
}

export const REMOVE_HISTORY_NOTE = "Branch and run history stay in the workspace.";

export const REMOVE_DOWNGRADE_NOTE =
  "Branch has unique local commits, but the branch also exists on a remote — safe to delete locally.";

export function boundIdleNote(idleSessions: number): string {
  return `${idleSessions} idle ${idleSessions === 1 ? "session" : "sessions"} bound to this worktree will be stopped. Its history remains readable in the workspace.`;
}
