import { Field, FieldError, FieldLabel, FormSection, Input, PillGroup } from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import {
  environmentSpecForChoice,
  INHERIT_ENVIRONMENT,
  loopEnvironmentItems,
  type LoopEnvironmentChoice,
} from "../../lib/loop-environment-choice";
import type { LoopEnvironmentSpec } from "../../types";

interface LoopWorktreeSectionProps {
  value: LoopEnvironmentSpec | null;
  worktrees: readonly WorktreePayload[];
  gitBacked: boolean;
  disabled?: boolean;
  onChange: (next: LoopEnvironmentSpec | null) => void;
}

const MODE_HINTS: Partial<Record<LoopEnvironmentChoice, string>> = {
  [INHERIT_ENVIRONMENT]: "No loop default declared — nodes choose their own environment.",
  per_run: "Fresh worktree per run.",
};

/**
 * The loop-level environment default.
 *
 * "Inherit" is the rendering of an absent key, so choosing it clears the stored
 * default rather than writing a placeholder mode. A node override always wins
 * over whatever is set here.
 */
export function LoopWorktreeSection({
  value,
  worktrees,
  gitBacked,
  disabled = false,
  onChange,
}: LoopWorktreeSectionProps) {
  const mode: LoopEnvironmentChoice = value?.mode ?? INHERIT_ENVIRONMENT;
  const items = loopEnvironmentItems(gitBacked, disabled);
  const hint = MODE_HINTS[mode];
  const invalidDirectory = value?.mode === "directory" && !value.directory?.trim();

  function handleModeChange(next: LoopEnvironmentChoice) {
    onChange(environmentSpecForChoice(next, value));
  }

  return (
    <FormSection title="Environment default">
      <div data-mode={mode} data-slot="loop-worktree-section">
        <PillGroup
          aria-label="Loop environment default"
          data-testid="loop-configure-environment-mode"
          items={items}
          onChange={handleModeChange}
          size="sm"
          value={mode}
        />
        {value?.mode === "worktree" ? (
          <div className="mt-2">
            <WorktreeRefSelect
              ariaLabel="Loop default worktree"
              disabled={disabled}
              onChange={worktreeRef => onChange({ mode: "worktree", worktree_ref: worktreeRef })}
              testId="loop-configure-environment-ref"
              value={value.worktree_ref ?? ""}
              worktrees={worktrees}
            />
          </div>
        ) : null}
        {value?.mode === "directory" ? (
          <Field className="mt-3" data-invalid={invalidDirectory ? "" : undefined}>
            <FieldLabel htmlFor="loop-configure-environment-directory">Directory</FieldLabel>
            <Input
              disabled={disabled}
              id="loop-configure-environment-directory"
              onChange={event => onChange({ mode: "directory", directory: event.target.value })}
              placeholder="packages/api"
              value={value.directory ?? ""}
            />
            {invalidDirectory ? (
              <FieldError>Enter a directory inside the workspace.</FieldError>
            ) : null}
          </Field>
        ) : null}
        {hint ? <p className="mt-2 text-form-hint leading-relaxed text-subtle">{hint}</p> : null}
      </div>
    </FormSection>
  );
}
