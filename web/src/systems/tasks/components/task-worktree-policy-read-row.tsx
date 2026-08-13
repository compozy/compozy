import { Pill, PropertyRow } from "@compozy/ui";

import { WorktreeStateChip, type WorktreePayload } from "@/systems/workspace";

import {
  isTaskWorktreeRefInvalid,
  TASK_WORKTREE_POLICY_MODE_LABELS,
} from "../lib/task-worktree-policy";
import type { TaskExecutionProfileWorktree } from "../types";

interface TaskWorktreePolicyReadRowProps {
  value: TaskExecutionProfileWorktree;
  worktrees?: readonly WorktreePayload[];
}

/**
 * Reads the policy back on the profile view. A `ref` policy shows the worktree's
 * record label — never its branch or the path basename — and flags a reference
 * that no longer resolves instead of presenting it as settled.
 */
export function TaskWorktreePolicyReadRow({ value, worktrees }: TaskWorktreePolicyReadRowProps) {
  const worktreeRef = value.worktree_ref ?? "";
  const referenced = worktrees?.find(worktree => worktree.id === worktreeRef);
  const invalid = isTaskWorktreeRefInvalid(value.mode, worktreeRef, worktrees ?? []);
  const invalidState =
    referenced?.state === "pending" ||
    referenced?.state === "failed" ||
    referenced?.state === "missing"
      ? referenced.state
      : "missing";

  return (
    <PropertyRow
      data-invalid={invalid ? "" : undefined}
      data-mode={value.mode}
      data-slot="task-worktree-policy-read-row"
      label="Worktree"
    >
      {value.mode === "ref" && worktreeRef !== "" ? (
        <span className="flex items-center gap-1.5">
          <Pill mono size="xs" tone="neutral">
            {referenced?.name ?? worktreeRef}
          </Pill>
          {invalid ? <WorktreeStateChip state={invalidState} /> : null}
        </span>
      ) : (
        <span className="text-small-body text-muted" data-slot="task-worktree-policy-read-value">
          {TASK_WORKTREE_POLICY_MODE_LABELS[value.mode]}
        </span>
      )}
    </PropertyRow>
  );
}
