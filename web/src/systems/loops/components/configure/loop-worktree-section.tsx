import { FormSection, PillGroup } from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import {
  LOOP_ENVIRONMENT_MODE_LABELS,
  LOOP_ENVIRONMENT_MODES,
} from "../../lib/loop-node-schema-types";
import type { LoopEnvironmentMode, LoopEnvironmentSpec } from "../../types";

interface LoopWorktreeSectionProps {
  value: LoopEnvironmentSpec | null;
  worktrees: readonly WorktreePayload[];
  gitBacked: boolean;
  disabled?: boolean;
  onChange: (next: LoopEnvironmentSpec | null) => void;
}

const INHERIT_VALUE = "__inherit__";
type EnvironmentChoice = LoopEnvironmentMode | typeof INHERIT_VALUE;

const MODE_HINTS: Partial<Record<EnvironmentChoice, string>> = {
  [INHERIT_VALUE]: "No loop default declared — nodes choose their own environment.",
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
  const mode: EnvironmentChoice = value?.mode ?? INHERIT_VALUE;
  const items = [
    { value: INHERIT_VALUE as EnvironmentChoice, label: "Inherit", disabled },
    ...LOOP_ENVIRONMENT_MODES.map(entry => ({
      value: entry as EnvironmentChoice,
      label: LOOP_ENVIRONMENT_MODE_LABELS[entry],
      disabled: disabled || (!gitBacked && (entry === "worktree" || entry === "per_run")),
    })),
  ];
  const hint = MODE_HINTS[mode];

  function handleModeChange(next: EnvironmentChoice) {
    if (next === INHERIT_VALUE) return onChange(null);
    if (next === "worktree") {
      return onChange({ mode: "worktree", worktree_ref: value?.worktree_ref ?? "" });
    }
    if (next === "directory") {
      return onChange({ mode: "directory", directory: value?.directory ?? "" });
    }
    return onChange({ mode: next });
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
        {hint ? <p className="mt-2 text-form-hint leading-relaxed text-subtle">{hint}</p> : null}
      </div>
    </FormSection>
  );
}
