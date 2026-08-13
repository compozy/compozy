import type { WorktreePayload } from "@/systems/workspace";

import type { TaskExecutionProfileWorktreeMode } from "../types";

export const TASK_WORKTREE_POLICY_MODE_LABELS: Record<TaskExecutionProfileWorktreeMode, string> = {
  inherit: "Inherit",
  none: "Workspace root",
  ref: "Named worktree",
  per_run: "Per-run",
};

/** A named policy is runnable only while its selected worktree is ready. */
export function isTaskWorktreeRefInvalid(
  mode: TaskExecutionProfileWorktreeMode,
  worktreeRef: string,
  worktrees: readonly WorktreePayload[]
): boolean {
  if (mode !== "ref") return false;
  const referenced = worktrees.find(worktree => worktree.id === worktreeRef);
  return referenced?.state !== "ready";
}
