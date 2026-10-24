import { Field, FieldLabel, FormSection, Input, PillGroup, Skeleton } from "@compozy/ui";

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
  const items = loopEnvironmentItems(gitBacked, disabled, mode);
  const hint = MODE_HINTS[mode];

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
          <Field className="mt-3">
            <FieldLabel htmlFor="loop-configure-environment-directory">Directory</FieldLabel>
            <Input
              disabled={disabled}
              id="loop-configure-environment-directory"
              readOnly
              value={value.directory ?? ""}
            />
          </Field>
        ) : null}
        {hint ? <p className="mt-2 text-form-hint leading-relaxed text-subtle">{hint}</p> : null}
      </div>
    </FormSection>
  );
}

/**
 * The environment default's mode picker while it is being read: the real section heading over
 * segment-shaped placeholders. Presentational, so the caller wraps the area in a `SkeletonRegion`.
 */
export function LoopWorktreeSectionSkeleton() {
  return (
    <FormSection title="Environment default">
      <div aria-hidden="true" className="inline-flex items-center gap-1">
        <Skeleton className="h-(--height-pill-group-segment-sm) w-16 rounded-pill" />
        <Skeleton className="h-(--height-pill-group-segment-sm) w-14 rounded-pill" />
        <Skeleton className="h-(--height-pill-group-segment-sm) w-20 rounded-pill" />
      </div>
    </FormSection>
  );
}
