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

function describedByIds(
  description: React.ReactNode,
  error: React.ReactNode,
  descriptionId: string,
  errorId: string
): string | undefined {
  // Plain id list — `cn` is a class merger and would mangle these.
  const ids = [description ? descriptionId : null, error ? errorId : null].filter(Boolean);
  return ids.length > 0 ? ids.join(" ") : undefined;
}

function scopedTestId(prefix: string | undefined, suffix: string): string | undefined {
  return prefix && `${prefix}-${suffix}`;
}

interface SecretFieldHeadProps {
  id: string;
  label: React.ReactNode;
  labelClassName?: string;
  badges?: React.ReactNode;
  description?: React.ReactNode;
  descriptionId: string;
  fieldName: string;
  mode: SecretFieldMode;
  onModeChange?: (mode: SecretFieldMode) => void;
  binding?: SecretFieldBinding;
  sourceModeLabel: React.ReactNode;
  showModePicker: boolean;
  testIdPrefix?: string;
}

/** Label row, description, and the value / stored-reference mode picker. */
function SecretFieldHead({
  id,
  label,
  labelClassName,
  badges,
  description,
  descriptionId,
  fieldName,
  mode,
  onModeChange,
  binding,
  sourceModeLabel,
  showModePicker,
  testIdPrefix,
}: SecretFieldHeadProps) {
  const locked = binding?.create?.open === true && binding.create.pending === true;
  const modeItems: PillGroupItem<SecretFieldMode>[] = [
    {
      value: "value",
      label: "Enter value",
      testId: scopedTestId(testIdPrefix, "mode-value"),
      disabled: locked,
    },
    {
      value: "source",
      label: sourceModeLabel,
      testId: scopedTestId(testIdPrefix, "mode-source"),
      disabled: locked,
    },
  ];
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <FieldLabel className={labelClassName} htmlFor={id}>
          {label}
        </FieldLabel>
        {badges}
      </div>
      {description ? <FieldDescription id={descriptionId}>{description}</FieldDescription> : null}
      {showModePicker && onModeChange ? (
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
  );
}

interface SecretFieldPresenceProps {
  presenceLabel?: React.ReactNode;
  rotated: boolean;
  saving: boolean;
  replaceLabel: string;
  onReplace: () => void;
  testIdPrefix?: string;
}

/** Stored-and-hidden summary with the explicit rotation entry point. */
function SecretFieldPresence({
  presenceLabel,
  rotated,
  saving,
  replaceLabel,
  onReplace,
  testIdPrefix,
}: SecretFieldPresenceProps) {
  return (
    <div
      className="flex flex-wrap items-center gap-2 rounded-md bg-sunken px-3 py-2"
      data-slot="secret-field-presence"
      data-testid={scopedTestId(testIdPrefix, "presence")}
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
        data-testid={scopedTestId(testIdPrefix, "replace")}
        disabled={saving}
        onClick={onReplace}
        size="sm"
        type="button"
        variant="outline"
      >
        {replaceLabel}
      </Button>
    </div>
  );
}

interface SecretFieldValueInputProps {
  id: string;
  value: string;
  onValueChange: (next: string) => void;
  onBlur?: () => void;
  placeholder?: string;
  describedBy?: string;
  invalid: boolean;
  required: boolean;
  saving: boolean;
  /** Rotating an existing value: show the cancel affordance that keeps it. */
  rotating: boolean;
  onCancelRotation: () => void;
  testIdPrefix?: string;
}

/** The single write-only input, plus the cancel row while rotating. */
function SecretFieldValueInput({
  id,
  value,
  onValueChange,
  onBlur,
  placeholder,
  describedBy,
  invalid,
  required,
  saving,
  rotating,
  onCancelRotation,
  testIdPrefix,
}: SecretFieldValueInputProps) {
  return (
    <div className="flex flex-col gap-2">
      <Input
        aria-describedby={describedBy}
        aria-invalid={invalid ? true : undefined}
        autoComplete="new-password"
        disabled={saving}
        id={id}
        onBlur={onBlur}
        onChange={event => onValueChange(event.target.value)}
        placeholder={placeholder ?? "Stored write-only"}
        required={required}
        type="password"
        value={value}
      />
      {rotating ? (
        <div className="flex items-center gap-2">
          <p className="mr-auto text-form-hint text-subtle">
            Cancel keeps the stored value in place.
          </p>
          <Button
            className={DIALOG_TOUCH_TARGET_CLASS}
            data-testid={scopedTestId(testIdPrefix, "cancel")}
            disabled={saving}
            onClick={onCancelRotation}
            size="sm"
            type="button"
            variant="ghost"
          >
            Cancel
          </Button>
        </div>
      ) : null}
    </div>
  );
}

/**
 * Write-only secret control.
 *
 * Plaintext enters once and is never read back: create renders a single write
 * input, edit renders presence plus an explicit rotation, and cancelling a
 * rotation restores the existing binding untouched. No Compozy secret read path
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
  const showPresenceSummary = present && !editing;
  const sourceBinding = binding && mode === "source" ? binding : null;

  const body = showPresenceSummary ? (
    <SecretFieldPresence
      onReplace={() => onEditingChange?.(true)}
      presenceLabel={presenceLabel}
      replaceLabel={replaceLabel}
      rotated={rotated}
      saving={saving}
      testIdPrefix={testIdPrefix}
    />
  ) : sourceBinding ? (
    <SecretFieldSources
      binding={sourceBinding}
      createTestId={createTestId ?? scopedTestId(testIdPrefix, "create")}
      fieldLabel={fieldName}
      testId={sourcesTestId ?? scopedTestId(testIdPrefix, "sources")}
    />
  ) : (
    <SecretFieldValueInput
      describedBy={describedByIds(description, error, descriptionId, errorId)}
      id={id}
      invalid={Boolean(error)}
      onBlur={onBlur}
      onCancelRotation={() => {
        onValueChange("");
        onEditingChange?.(false);
      }}
      onValueChange={onValueChange}
      placeholder={placeholder}
      required={required && !present}
      rotating={present && editing}
      saving={saving}
      testIdPrefix={testIdPrefix}
      value={value}
    />
  );

  return (
    <Field
      className={className}
      data-invalid={error ? "true" : undefined}
      data-slot="secret-field"
      data-state={state}
      data-testid={testIdPrefix}
    >
      <SecretFieldHead
        badges={badges}
        binding={binding}
        description={description}
        descriptionId={descriptionId}
        fieldName={fieldName}
        id={id}
        label={label}
        labelClassName={labelClassName}
        mode={mode}
        onModeChange={onModeChange}
        showModePicker={Boolean(binding) && !showPresenceSummary}
        sourceModeLabel={sourceModeLabel}
        testIdPrefix={testIdPrefix}
      />
      {body}
      {error ? <FieldError id={errorId}>{error}</FieldError> : null}
    </Field>
  );
}

export { SecretField };
