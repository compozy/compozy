"use client";

import * as React from "react";

import {
  DIALOG_TOUCH_TARGET_CLASS,
  DIALOG_TOUCH_TARGET_SEGMENTS_CLASS,
} from "../../lib/dialog-shell";
import { cn } from "../../lib/utils";
import { Button } from "../button";
import { Field, FieldDescription, FieldError, FieldLabel } from "../field";
import { Input } from "../input";
import { Pill } from "./pill";
import { PillGroup, type PillGroupItem } from "./pill-group";
import { SecretFieldSources, type SecretFieldBinding } from "./secret-field-sources";

export type {
  SecretFieldBinding,
  SecretFieldSource,
  SecretFieldSourceCreate,
} from "./secret-field-sources";

/** Normative secret lifecycle — `STATE-MATRIX.md` § SecretField. */
export type SecretFieldState = "absent" | "present" | "editing" | "invalid" | "saving" | "rotated";

/** Whether the field carries a typed plaintext value or binds a stored reference. */
export type SecretFieldMode = "value" | "source";

export interface SecretFieldProps {
  id: string;
  label: React.ReactNode;
  description?: React.ReactNode;
  required?: boolean;
  /** Trailing markers beside the label (`secret`, `required`, …). */
  badges?: React.ReactNode;
  labelClassName?: string;

  /** Write-only draft. Never populated from a read path. */
  value: string;
  onValueChange: (next: string) => void;
  onBlur?: () => void;
  placeholder?: string;

  /** Presence reported by the read path — presence only, never plaintext. */
  present?: boolean;
  /** Summary of what is stored, e.g. the reference it was written under. */
  presenceLabel?: React.ReactNode;
  editing?: boolean;
  onEditingChange?: (editing: boolean) => void;
  replaceLabel?: string;

  saving?: boolean;
  /** Confirms a completed rotation without echoing the new value. */
  rotated?: boolean;
  error?: React.ReactNode;

  mode?: SecretFieldMode;
  onModeChange?: (mode: SecretFieldMode) => void;
  /** Enables binding to an existing stored reference instead of typing a value. */
  binding?: SecretFieldBinding;
  /** Runtime-truthful label for the binding mode, e.g. `Use Vault`. */
  sourceModeLabel?: React.ReactNode;

  testIdPrefix?: string;
  /** Overrides the derived `${testIdPrefix}-sources` id. */
  sourcesTestId?: string;
  /** Overrides the derived `${testIdPrefix}-create` id. */
  createTestId?: string;
  className?: string;
}

function resolveState({
  saving,
  error,
  rotated,
  present,
  editing,
}: Pick<
  SecretFieldProps,
  "saving" | "error" | "rotated" | "present" | "editing"
>): SecretFieldState {
  if (saving) return "saving";
  if (error) return "invalid";
  if (rotated) return "rotated";
  if (!present) return "absent";
  return editing ? "editing" : "present";
}

/**
 * Write-only secret control.
 *
 * Plaintext enters once and is never read back: create renders a single write
 * input, edit renders presence plus an explicit rotation, and cancelling a
 * rotation restores the existing binding untouched. No AGH secret read path
 * returns a value, so this component never prefills from a GET.
 */
function SecretField({
  id,
  label,
  description,
  required = false,
  badges,
  labelClassName,
  value,
  onValueChange,
  onBlur,
  placeholder,
  present = false,
  presenceLabel,
  editing = false,
  onEditingChange,
  replaceLabel = "Replace",
  saving = false,
  rotated = false,
  error,
  mode = "value",
  onModeChange,
  binding,
  sourceModeLabel = "Use stored secret",
  testIdPrefix,
  sourcesTestId,
  createTestId,
  className,
}: SecretFieldProps) {
  const state = resolveState({ saving, error, rotated, present, editing });
  /** Accessible names read better from the visible label than from the input id. */
  const fieldName = typeof label === "string" ? label : id;
  const descriptionId = `${id}-description`;
  const errorId = `${id}-error`;
  // Plain id list — `cn` is a class merger and would mangle these.
  const describedBy =
    [description ? descriptionId : null, error ? errorId : null].filter(Boolean).join(" ") ||
    undefined;
  const showPresenceSummary = present && !editing;
  const sourceBinding = binding && mode === "source" ? binding : null;
  const bindingModeLocked = binding?.create?.open === true && binding.create.pending === true;
  const modeItems: PillGroupItem<SecretFieldMode>[] = [
    {
      value: "value",
      label: "Enter value",
      testId: testIdPrefix && `${testIdPrefix}-mode-value`,
      disabled: bindingModeLocked,
    },
    {
      value: "source",
      label: sourceModeLabel,
      testId: testIdPrefix && `${testIdPrefix}-mode-source`,
      disabled: bindingModeLocked,
    },
  ];

  const cancelRotation = () => {
    onValueChange("");
    onEditingChange?.(false);
  };

  return (
    <Field
      className={className}
      data-invalid={error ? "true" : undefined}
      data-slot="secret-field"
      data-state={state}
      data-testid={testIdPrefix}
    >
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <FieldLabel className={labelClassName} htmlFor={id}>
            {label}
          </FieldLabel>
          {badges}
        </div>
        {description ? <FieldDescription id={descriptionId}>{description}</FieldDescription> : null}
        {binding && onModeChange && !showPresenceSummary ? (
          <PillGroup
            aria-label={`${fieldName} binding method`}
            className={cn("grid w-full grid-cols-2", DIALOG_TOUCH_TARGET_SEGMENTS_CLASS)}
            items={modeItems}
            onChange={onModeChange}
            size="sm"
            value={mode}
          />
        ) : null}
      </div>

      {showPresenceSummary ? (
        <div
          className="flex flex-wrap items-center gap-2 rounded-md bg-canvas px-3 py-2"
          data-slot="secret-field-presence"
          data-testid={testIdPrefix && `${testIdPrefix}-presence`}
        >
          <span aria-hidden="true" className="font-mono text-mono-id tracking-normal text-muted">
            ••••••••
          </span>
          <span className="sr-only">Value stored and hidden</span>
          {presenceLabel ? (
            <span className="min-w-0 truncate text-form-hint text-muted">{presenceLabel}</span>
          ) : null}
          {rotated ? (
            <Pill size="xs" tone="success">
              rotated
            </Pill>
          ) : null}
          <Button
            className={cn("ml-auto", DIALOG_TOUCH_TARGET_CLASS)}
            data-testid={testIdPrefix && `${testIdPrefix}-replace`}
            disabled={saving}
            onClick={() => onEditingChange?.(true)}
            size="sm"
            type="button"
            variant="outline"
          >
            {replaceLabel}
          </Button>
        </div>
      ) : sourceBinding ? (
        <SecretFieldSources
          binding={sourceBinding}
          createTestId={createTestId ?? (testIdPrefix && `${testIdPrefix}-create`)}
          fieldLabel={fieldName}
          testId={sourcesTestId ?? (testIdPrefix && `${testIdPrefix}-sources`)}
        />
      ) : (
        <div className="flex flex-col gap-2">
          <Input
            aria-describedby={describedBy}
            aria-invalid={error ? true : undefined}
            autoComplete="new-password"
            disabled={saving}
            id={id}
            onBlur={onBlur}
            onChange={event => onValueChange(event.target.value)}
            placeholder={placeholder ?? "Stored write-only"}
            required={required && !present}
            type="password"
            value={value}
          />
          {present && editing ? (
            <div className="flex items-center gap-2">
              <p className="mr-auto text-form-hint text-subtle">
                Cancel keeps the stored value in place.
              </p>
              <Button
                className={DIALOG_TOUCH_TARGET_CLASS}
                data-testid={testIdPrefix && `${testIdPrefix}-cancel`}
                disabled={saving}
                onClick={cancelRotation}
                size="sm"
                type="button"
                variant="ghost"
              >
                Cancel
              </Button>
            </div>
          ) : null}
        </div>
      )}

      {error ? <FieldError id={errorId}>{error}</FieldError> : null}
    </Field>
  );
}

export { SecretField };
