import { PlusIcon } from "lucide-react";

import { Button, CommandItem, Field, FieldDescription, FieldError, FieldLabel } from "@compozy/ui";

import {
  WorktreeRefSelect,
  WorktreeStateChip,
  type WorktreeMaterialization,
  type WorktreePayload,
} from "@/systems/workspace";

import {
  isEnvironmentTargetMissing,
  NEW_WORKTREE_LABEL,
  WORKSPACE_ROOT_LABEL,
  type SessionEnvironmentTarget,
} from "../lib/session-environment-target";

interface SessionEnvironmentFieldProps {
  value: SessionEnvironmentTarget;
  worktrees: readonly WorktreePayload[];
  onChange: (next: SessionEnvironmentTarget) => void;
  /** Live creation of the "New worktree…" target, when one is in flight. */
  materialization?: WorktreeMaterialization;
  disabled?: boolean;
}

const ROOT_VALUE = "__root__";

/**
 * Where a new session will run.
 *
 * The control is absent — never disabled — on a workspace git cannot back,
 * because there is no environment to choose there. A selection that stops being
 * ready blocks submission instead of silently falling back to the root.
 */
export function SessionEnvironmentField({
  value,
  worktrees,
  onChange,
  materialization,
  disabled = false,
}: SessionEnvironmentFieldProps) {
  const missing = isEnvironmentTargetMissing(value, worktrees);
  const missingName =
    value.kind === "worktree"
      ? (worktrees.find(worktree => worktree.id === value.worktreeId)?.name ?? value.worktreeId)
      : "";
  const pending = materialization?.status === "pending" || materialization?.status === "creating";
  const state = missing ? "missing" : pending ? "pending" : value.kind;

  return (
    <Field
      data-pending={pending ? "" : undefined}
      data-slot="session-environment-field"
      data-state={state}
    >
      <FieldLabel htmlFor="session-environment">Environment</FieldLabel>
      <WorktreeRefSelect
        ariaLabel="Environment"
        describedBy={
          missing || materialization?.status === "failed" ? "session-environment-error" : undefined
        }
        disabled={disabled || pending}
        displayValue={value.kind === "new" ? NEW_WORKTREE_LABEL : undefined}
        emptyOption={{ value: ROOT_VALUE, label: WORKSPACE_ROOT_LABEL }}
        footer={
          <CommandItem
            onSelect={() =>
              onChange({
                kind: "new",
                name: "",
                previous:
                  value.kind === "worktree"
                    ? { kind: "worktree", worktreeId: value.worktreeId }
                    : { kind: "root" },
              })
            }
            value="new worktree"
          >
            <PlusIcon aria-hidden="true" className="size-3" />
            {NEW_WORKTREE_LABEL}
          </CommandItem>
        }
        invalid={missing || materialization?.status === "failed"}
        onChange={next =>
          onChange(next === ROOT_VALUE ? { kind: "root" } : { kind: "worktree", worktreeId: next })
        }
        testId="session-create-environment"
        triggerId="session-environment"
        value={value.kind === "worktree" ? value.worktreeId : ROOT_VALUE}
        worktrees={worktrees}
      />
      {pending ? (
        <FieldDescription className="flex items-center gap-2" data-slot="session-environment-phase">
          <WorktreeStateChip state="pending" />
          <span className="font-mono text-mono-id text-subtle">{materialization?.phase ?? ""}</span>
          <Button
            data-testid="session-environment-cancel"
            onClick={() => {
              materialization?.cancel();
              materialization?.recover();
              onChange(value.kind === "new" ? (value.previous ?? { kind: "root" }) : value);
            }}
            size="sm"
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
        </FieldDescription>
      ) : null}
      {missing ? (
        <FieldError id="session-environment-error">
          {`Worktree ${missingName} is no longer available. Choose another environment.`}
        </FieldError>
      ) : null}
      {materialization?.status === "failed" && materialization.error ? (
        <FieldError className="flex flex-wrap items-center gap-2" id="session-environment-error">
          <span>{materialization.error}</span>
          <Button
            data-testid="session-environment-retry"
            onClick={materialization.retry}
            size="sm"
            type="button"
            variant="ghost"
          >
            Retry
          </Button>
          <Button
            data-testid="session-environment-root"
            onClick={() => {
              materialization.recover();
              onChange({ kind: "root" });
            }}
            size="sm"
            type="button"
            variant="ghost"
          >
            Use workspace root
          </Button>
        </FieldError>
      ) : null}
    </Field>
  );
}
