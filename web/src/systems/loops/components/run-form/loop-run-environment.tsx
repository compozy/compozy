import { FolderGit2 } from "lucide-react";

import { Field, FieldError, FieldLabel, Input, PillGroup } from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import {
  environmentSpecForChoice,
  INHERIT_ENVIRONMENT,
  loopEnvironmentItems,
  type LoopEnvironmentChoice,
} from "../../lib/loop-environment-choice";
import { environmentGist } from "../../lib/loop-run-form";
import type { LoopEnvironmentSpec } from "../../types";
import { LoopRailSection } from "../loop-rail-section";

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
  const items = loopEnvironmentItems(gitBacked, disabled, choice);

  function handleModeChange(next: LoopEnvironmentChoice) {
    onChange(environmentSpecForChoice(next, value));
  }

  return (
    <LoopRailSection
      gist={environmentGist(value)}
      icon={<FolderGit2 aria-hidden="true" className="size-3.5" />}
      title="Environment"
    >
      <div className="px-3.5 py-3" data-mode={choice} data-slot="loop-run-environment">
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
          <Field className="mt-3">
            <FieldLabel htmlFor="loop-run-environment-directory">Directory</FieldLabel>
            <Input
              disabled={disabled}
              id="loop-run-environment-directory"
              readOnly
              value={value.directory ?? ""}
            />
          </Field>
        ) : null}
      </div>
    </LoopRailSection>
  );
}
