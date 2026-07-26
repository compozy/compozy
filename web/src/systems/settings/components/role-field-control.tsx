import type { ReactNode } from "react";

import { cn, Input, Switch } from "@agh/ui";

import type { RoleFieldDescriptor } from "../lib/roles-config";
import { RoleEffectiveHint } from "./role-effective-hint";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsNumberInput } from "./settings-number-input";

interface RoleFieldControlProps {
  field: RoleFieldDescriptor;
  value: string | number | boolean;
  /** Whether this field carries an effective (projected) value hint. */
  hasEffective: boolean;
  /** Projected effective value; `null` means "resolves at invocation" (never fabricated). */
  effective: string | null;
  error?: string;
  disabled?: boolean;
  testId: string;
  resetRevision?: number;
  fieldRef?: (element: HTMLInputElement | null) => void;
  onValueChange: (value: string | number | boolean) => void;
  onValidityChange?: (message: string | null) => void;
}

function renderControl({
  field,
  value,
  error,
  disabled,
  testId,
  onValueChange,
  onValidityChange,
  resetRevision,
  fieldRef,
}: RoleFieldControlProps): ReactNode {
  switch (field.kind) {
    case "switch":
      return (
        <Switch
          data-testid={`${testId}-switch`}
          checked={Boolean(value)}
          disabled={disabled}
          onCheckedChange={checked => onValueChange(checked)}
        />
      );
    case "number":
      return (
        <SettingsNumberInput
          className="w-24"
          data-testid={`${testId}-input`}
          min={field.min}
          value={Number(value)}
          resetRevision={resetRevision}
          ref={fieldRef}
          disabled={disabled}
          onValueChange={next => onValueChange(next)}
          onValidityChange={onValidityChange}
        />
      );
    default:
      return (
        <Input
          className={cn("w-56", field.mono && "font-mono")}
          data-testid={`${testId}-input`}
          value={String(value)}
          placeholder={field.placeholder}
          disabled={disabled}
          aria-invalid={error ? true : undefined}
          onChange={event => onValueChange(event.target.value)}
        />
      );
  }
}

/** One editable role policy field rendered as a settings row. */
export function RoleFieldControl(props: RoleFieldControlProps) {
  const { field, hasEffective, effective, error, testId } = props;
  const description = (
    <>
      {field.description ? <span>{field.description}</span> : null}
      {hasEffective ? (
        <RoleEffectiveHint effective={effective} emptyLabel="Resolves at invocation." />
      ) : null}
    </>
  );
  return (
    <SettingsFieldRow
      data-testid={testId}
      label={field.label}
      description={description}
      error={error}
      control={renderControl(props)}
    />
  );
}
