import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  PillGroup,
  type PillGroupItem,
} from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import {
  isTaskWorktreeRefInvalid,
  TASK_WORKTREE_POLICY_MODE_LABELS,
} from "../lib/task-worktree-policy";
import type { TaskExecutionProfileWorktree, TaskExecutionProfileWorktreeMode } from "../types";

interface TaskWorktreePolicyFieldsProps {
  value: TaskExecutionProfileWorktree;
  worktrees: readonly WorktreePayload[];
  workspacePath?: string;
  /** Locked while a run is active — the policy is read at claim time. */
  locked?: boolean;
  pending?: boolean;
  onChange: (next: TaskExecutionProfileWorktree) => void;
}

/** The locked environment-mode vocabulary. Never "None", "Unset", or "Root". */
const MODE_ITEMS: ReadonlyArray<PillGroupItem<TaskExecutionProfileWorktreeMode>> = [
  { value: "inherit", label: TASK_WORKTREE_POLICY_MODE_LABELS.inherit },
  { value: "none", label: TASK_WORKTREE_POLICY_MODE_LABELS.none },
  { value: "ref", label: TASK_WORKTREE_POLICY_MODE_LABELS.ref },
  { value: "per_run", label: TASK_WORKTREE_POLICY_MODE_LABELS.per_run },
];

const PER_RUN_HINT =
  "Each run gets a fresh worktree on a run/ branch. Work is preserved after the run ends; cleanup is suggested only when provably safe.";

/**
 * The task's worktree policy.
 *
 * Every change is written through the dedicated policy patch, not the profile
 * replace — so changing where runs execute can never carry a stale copy of an
 * unrelated block along with it.
 */
export function TaskWorktreePolicyFields({
  value,
  worktrees,
  workspacePath,
  locked = false,
  pending = false,
  onChange,
}: TaskWorktreePolicyFieldsProps) {
  const disabled = locked || pending;
  const selectedRef = value.worktree_ref ?? "";
  const referenced = worktrees.find(worktree => worktree.id === selectedRef);
  const invalidRef = isTaskWorktreeRefInvalid(value.mode, selectedRef, worktrees);
  const hint =
    value.mode === "none" && workspacePath
      ? `Runs at the workspace root ${workspacePath} regardless of other defaults.`
      : value.mode === "per_run"
        ? PER_RUN_HINT
        : undefined;

  return (
    <Field
      data-invalid={invalidRef ? "" : undefined}
      data-locked={locked ? "" : undefined}
      data-mode={value.mode}
      data-slot="task-worktree-policy"
    >
      <FieldLabel htmlFor="tasks-setup-worktree-mode">Worktree</FieldLabel>
      <PillGroup
        aria-label="Worktree policy"
        data-testid="tasks-setup-worktree-mode"
        id="tasks-setup-worktree-mode"
        items={MODE_ITEMS.map(item => ({ ...item, disabled }))}
        onChange={mode => onChange(mode === "ref" ? { mode, worktree_ref: selectedRef } : { mode })}
        size="sm"
        value={value.mode}
      />
      {value.mode === "ref" ? (
        <WorktreeRefSelect
          ariaLabel="Task worktree"
          disabled={disabled}
          onChange={worktreeRef => onChange({ mode: "ref", worktree_ref: worktreeRef })}
          testId="tasks-setup-worktree-ref"
          value={selectedRef}
          worktrees={worktrees}
        />
      ) : null}
      {hint ? <FieldDescription>{hint}</FieldDescription> : null}
      {invalidRef ? (
        <FieldError>
          {selectedRef === ""
            ? "Pick a ready worktree before saving this policy."
            : `Worktree ${referenced?.name ?? selectedRef} is not ready — runs will fail until you pick another.`}
        </FieldError>
      ) : null}
    </Field>
  );
}
