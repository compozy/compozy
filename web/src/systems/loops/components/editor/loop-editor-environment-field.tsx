import type { ComponentProps } from "react";

import { Field, FieldError, FieldLabel, Input, PillGroup, cn } from "@compozy/ui";

import { WorktreeRefSelect, type WorktreePayload } from "@/systems/workspace";

import type { RawLoopNode } from "../../lib/codec";
import { getAtPath, type NodeFieldEdit } from "../../lib/loop-editor-draft";
import { INHERIT_ENVIRONMENT, loopEnvironmentItems } from "../../lib/loop-environment-choice";
import {
  ENVIRONMENT_CWD_REMOVED_CODE,
  ENVIRONMENT_CWD_REMOVED_MESSAGE,
  environmentHint,
  environmentModeEdits,
  hasRetiredCwd,
} from "../../lib/loop-node-environment-fields";
import {
  LOOP_ENVIRONMENT_MODES,
  type EnvironmentFieldSpec,
  type FieldPath,
} from "../../lib/loop-node-schema-types";
import type { LoopEnvironmentSpec, LoopValidationIssue } from "../../types";

const UNSET_MODE_VALUE = "";

interface LoopEditorEnvironmentFieldProps extends Omit<ComponentProps<"div">, "onChange"> {
  field: EnvironmentFieldSpec;
  raw: RawLoopNode;
  disabled: boolean;
  onChangeFields: (edits: NodeFieldEdit[]) => void;
  onChange: (path: FieldPath, value: unknown) => void;
  worktrees?: readonly WorktreePayload[];
  gitBacked?: boolean;
  loopDefaultEnvironment?: LoopEnvironmentSpec;
  lintIssues?: readonly LoopValidationIssue[];
}

/**
 * The single execution-environment control on agent-executing nodes.
 *
 * Like the `wait` discriminator it is not a plain select: each mode forbids a
 * different companion key, so a mode change emits every edit at once and the
 * draft applies them in one transition — no rejected intermediate shape is ever
 * observable. Choosing Inherit clears the descriptor entirely, which is what
 * "follow the loop default" means on the wire.
 */
export function LoopEditorEnvironmentField({
  field,
  raw,
  disabled,
  onChangeFields,
  onChange,
  worktrees = [],
  gitBacked = true,
  loopDefaultEnvironment,
  lintIssues = [],
  className,
  ...props
}: LoopEditorEnvironmentFieldProps) {
  const cwdIssue = lintIssues.find(issue => issue.code === ENVIRONMENT_CWD_REMOVED_CODE);
  const invalidCwd = Boolean(cwdIssue) || hasRetiredCwd(raw);
  const mode = field.mode ?? INHERIT_ENVIRONMENT;
  const items = loopEnvironmentItems(gitBacked, disabled, mode).map(item =>
    item.value === INHERIT_ENVIRONMENT ? { ...item, label: field.inheritLabel } : item
  );
  const selectedRef = String(getAtPath(raw, [...field.basePath, "worktree_ref"]) ?? "");
  const referenced = worktrees.find(worktree => worktree.id === selectedRef);
  const readyWorktrees = worktrees.filter(worktree => worktree.state === "ready");
  const directory = String(getAtPath(raw, [...field.basePath, "directory"]) ?? "");
  const readOnlyMode = field.mode === "directory" || field.mode === "per_run";
  const hint =
    invalidCwd || readOnlyMode ? undefined : environmentHint(field.mode, loopDefaultEnvironment);

  function handleChange(next: string) {
    if (next === UNSET_MODE_VALUE) return;
    if (next === INHERIT_ENVIRONMENT) {
      onChangeFields(environmentModeEdits(raw, null));
      return;
    }
    const mode = LOOP_ENVIRONMENT_MODES.find(candidate => candidate === next);
    if (!mode) return;
    onChangeFields(environmentModeEdits(raw, mode));
  }

  return (
    <Field
      className={cn(className)}
      data-invalid={invalidCwd ? "" : undefined}
      data-mode={invalidCwd ? undefined : (field.mode ?? "inherit")}
      data-slot="loop-node-environment-field"
      {...props}
    >
      <span className="mb-1.5 flex items-center gap-1.5 text-form-label font-medium text-fg-strong">
        {field.label}
      </span>
      <PillGroup
        aria-label={field.label}
        data-testid={`loop-field-${field.key}`}
        items={items}
        onChange={handleChange}
        size="sm"
        value={invalidCwd ? UNSET_MODE_VALUE : (field.mode ?? INHERIT_ENVIRONMENT)}
      />
      {field.mode === "worktree" && !invalidCwd ? (
        <div className="mt-2">
          <WorktreeRefSelect
            ariaLabel="Node worktree"
            disabled={disabled}
            displayValue={referenced?.name}
            onChange={worktreeRef => onChange([...field.basePath, "worktree_ref"], worktreeRef)}
            testId="loop-field-environment-ref"
            value={selectedRef}
            worktrees={readyWorktrees}
          />
        </div>
      ) : null}
      {field.mode === "directory" && !invalidCwd ? (
        <div className="mt-3">
          <FieldLabel htmlFor="loop-field-environment-directory">Directory</FieldLabel>
          <Input
            disabled={disabled}
            id="loop-field-environment-directory"
            readOnly
            className="font-mono"
            data-testid="loop-field-environment-directory"
            value={directory}
          />
        </div>
      ) : null}
      {hint ? <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{hint}</p> : null}
      {invalidCwd ? (
        <FieldError>{cwdIssue?.message ?? ENVIRONMENT_CWD_REMOVED_MESSAGE}</FieldError>
      ) : null}
    </Field>
  );
}
