import type { WorktreeBranchOutcome } from "../types";

/** Plain-language reasons for a branch the daemon kept after an opted-in deletion. */
const KEPT_REASONS: Record<string, string> = {
  checkout_present: "a directory is back at the worktree path",
  root_checkout_branch: "the main checkout has it checked out",
  default_branch: "it is the default branch",
  registration_present: "Git still registers the worktree",
  checked_out_elsewhere: "another worktree has it checked out",
  not_fully_merged: "it has unmerged commits",
  delete_failed: "Git refused the deletion",
};

/** One sentence reporting what happened to the local branch. */
export function describeBranchOutcome(outcome: WorktreeBranchOutcome): string {
  if (!outcome.name || outcome.reason === "no_branch")
    return "The worktree had no branch to delete.";
  const name = outcome.name;
  if (outcome.deleted) return `Branch ${name} deleted.`;
  if (outcome.reason === "branch_absent") return `Branch ${name} was already gone.`;
  const reason = KEPT_REASONS[outcome.reason ?? ""] ?? "Git refused the deletion";
  return `Branch ${name} kept: ${reason}.`;
}
