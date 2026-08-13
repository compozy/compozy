import { Field, FieldError, FieldLabel, FormSection, Input, PillGroup } from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import {
  LOOP_ENVIRONMENT_MODE_LABELS,
  LOOP_ENVIRONMENT_MODES,
} from "../../lib/loop-node-schema-types";
import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../../types";

interface LoopRunEnvironmentProps {
  value: LoopEnvironmentSpec | null;
  worktrees: readonly WorktreePayload[];
  gitBacked: boolean;
  disabled?: boolean;
  onChange: (next: LoopEnvironmentSpec | null) => void;
}

const INHERIT_VALUE = "__inherit__";
type EnvironmentChoice = LoopEnvironmentMode | typeof INHERIT_VALUE;

/** One explicit environment override for this run; `Inherit` emits no override. */
export function LoopRunEnvironment({
  value,
  worktrees,
  gitBacked,
  disabled = false,
  onChange,
}: LoopRunEnvironmentProps) {
  const choice = value?.mode ?? INHERIT_VALUE;
  const invalidWorktree = value?.mode === "worktree" && !value.worktree_ref?.trim();
  const invalidDirectory = value?.mode === "directory" && !value.directory?.trim();
  const items = [
    { value: INHERIT_VALUE as EnvironmentChoice, label: "Inherit", disabled },
    ...LOOP_ENVIRONMENT_MODES.map(mode => ({
      value: mode as EnvironmentChoice,
      label: LOOP_ENVIRONMENT_MODE_LABELS[mode],
      disabled: disabled || (!gitBacked && (mode === "worktree" || mode === "per_run")),
    })),
  ];

  function handleModeChange(next: EnvironmentChoice) {
    if (next === INHERIT_VALUE) return onChange(null);
    if (next === "worktree") {
      return onChange({ mode: next, worktree_ref: value?.worktree_ref ?? "" });
    }
    if (next === "directory") {
      return onChange({ mode: next, directory: value?.directory ?? "" });
    }
    return onChange({ mode: next });
  }

  return (
    <FormSection rightLabel="this run only" title="Environment">
      <div data-mode={choice} data-slot="loop-run-environment">
        <PillGroup
          aria-label="Run environment"
          data-testid="loop-run-environment-mode"
          items={items}
          onChange={handleModeChange}
          size="sm"
          value={choice}
        />
        {value?.mode === "worktree" ? (
          <Field className="mt-3" data-invalid={invalidWorktree ? "" : undefined}>
            <FieldLabel>Worktree</FieldLabel>
            <WorktreeRefSelect
              ariaLabel="Run worktree"
              disabled={disabled}
              onChange={worktreeRef => onChange({ mode: "worktree", worktree_ref: worktreeRef })}
              testId="loop-run-environment-ref"
              value={value.worktree_ref ?? ""}
              worktrees={worktrees}
            />
            {invalidWorktree ? <FieldError>Pick a ready worktree.</FieldError> : null}
          </Field>
        ) : null}
        {value?.mode === "directory" ? (
          <Field className="mt-3" data-invalid={invalidDirectory ? "" : undefined}>
            <FieldLabel htmlFor="loop-run-environment-directory">Directory</FieldLabel>
            <Input
              disabled={disabled}
              id="loop-run-environment-directory"
              onChange={event => onChange({ mode: "directory", directory: event.target.value })}
              placeholder="packages/api"
              value={value.directory ?? ""}
            />
            {invalidDirectory ? (
              <FieldError>Enter a directory inside the workspace.</FieldError>
            ) : null}
          </Field>
        ) : null}
      </div>
    </FormSection>
  );
}
