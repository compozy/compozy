import { Field, FieldError, FieldLabel, FormSection, Input, PillGroup } from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import {
  environmentSpecForChoice,
  INHERIT_ENVIRONMENT,
  loopEnvironmentItems,
  type LoopEnvironmentChoice,
} from "../../lib/loop-environment-choice";
import type { LoopEnvironmentSpec } from "../../types";

interface LoopRunEnvironmentProps {
  value: LoopEnvironmentSpec | null;
  worktrees: readonly WorktreePayload[];
  gitBacked: boolean;
  disabled?: boolean;
  onChange: (next: LoopEnvironmentSpec | null) => void;
}

/** One explicit environment override for this run; `Inherit` emits no override. */
export function LoopRunEnvironment({
  value,
  worktrees,
  gitBacked,
  disabled = false,
  onChange,
}: LoopRunEnvironmentProps) {
  const choice = value?.mode ?? INHERIT_ENVIRONMENT;
  const invalidWorktree = value?.mode === "worktree" && !value.worktree_ref?.trim();
  const invalidDirectory = value?.mode === "directory" && !value.directory?.trim();
  const items = loopEnvironmentItems(gitBacked, disabled);

  function handleModeChange(next: LoopEnvironmentChoice) {
    onChange(environmentSpecForChoice(next, value));
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
