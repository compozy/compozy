import { useState } from "react";

import { useWorktrees, type WorktreePayload } from "@/systems/workspace";

import { isTaskWorktreeRefInvalid } from "../lib/task-worktree-policy";
import type { TaskExecutionProfile, TaskExecutionProfileWorktree } from "../types";
import { useSetTaskWorktreePolicy } from "./use-task-worktree-policy";

interface UseTaskWorktreePolicyControlParams {
  taskId: string;
  workspaceId: string | undefined;
  workspacePath?: string;
  profile: TaskExecutionProfile | null;
  hasActiveRun: boolean;
  enabled?: boolean;
}

export interface TaskWorktreePolicyControl {
  value: TaskExecutionProfileWorktree;
  worktrees: readonly WorktreePayload[];
  workspacePath?: string;
  locked: boolean;
  pending: boolean;
  valid: boolean;
  onChange: (next: TaskExecutionProfileWorktree) => void;
}

const INHERIT: TaskExecutionProfileWorktree = { mode: "inherit" };

interface PolicyDraft {
  taskId: string;
  serverKey: string;
  value: TaskExecutionProfileWorktree;
}

function policyKey(value: TaskExecutionProfileWorktree): string {
  return `${value.mode}\u0000${value.worktree_ref ?? ""}`;
}

/**
 * The policy control's view model, or `undefined` when there is nothing to
 * choose from — no workspace resolved yet, or a workspace git cannot back.
 * Returning `undefined` is how the surface stays absent rather than rendering a
 * control the runtime could never honour.
 */
export function useTaskWorktreePolicyControl({
  taskId,
  workspaceId,
  workspacePath,
  profile,
  hasActiveRun,
  enabled = true,
}: UseTaskWorktreePolicyControlParams): TaskWorktreePolicyControl | undefined {
  const worktrees = useWorktrees(workspaceId ?? null, {
    enabled: enabled && Boolean(workspaceId),
  });
  const mutation = useSetTaskWorktreePolicy();
  const serverValue = profile?.worktree ?? INHERIT;
  const serverKey = policyKey(serverValue);
  const [optimisticDraft, setOptimisticDraft] = useState<PolicyDraft | null>(null);
  const draft =
    enabled && optimisticDraft?.taskId === taskId && optimisticDraft.serverKey === serverKey
      ? optimisticDraft.value
      : serverValue;

  if (!workspaceId || !worktrees.data?.repo.git_backed) return undefined;

  const invalid = isTaskWorktreeRefInvalid(
    draft.mode,
    draft.worktree_ref ?? "",
    worktrees.data.worktrees
  );

  return {
    value: draft,
    worktrees: worktrees.data.worktrees,
    workspacePath,
    locked: hasActiveRun,
    pending: mutation.isPending,
    valid: !invalid,
    onChange: next => {
      setOptimisticDraft({ taskId, serverKey, value: next });
      if (next.mode === "ref" && !next.worktree_ref?.trim()) return;
      mutation.mutate(
        {
          id: taskId,
          data:
            next.mode === "ref"
              ? { mode: "ref", worktree_ref: next.worktree_ref }
              : { mode: next.mode },
        },
        {
          onError: () =>
            setOptimisticDraft(current => (current?.taskId === taskId ? null : current)),
        }
      );
    },
  };
}
