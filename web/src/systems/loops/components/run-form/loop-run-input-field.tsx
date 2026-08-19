import { AlertCircle } from "lucide-react";

import { RequiredMark } from "@compozy/ui";

import type { LoopInputSchemaField } from "../../types";
import { LoopTypedInputControl } from "../input/loop-typed-input-control";
import { MonoTag } from "../mono-tag";

interface LoopRunInputFieldProps {
  name: string;
  field: LoopInputSchemaField;
  value: unknown;
  /** Inline required error, surfaced only after a submit attempt. */
  error?: string;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}

export function LoopRunInputField({
  name,
  field,
  value,
  error,
  disabled,
  onChange,
}: LoopRunInputFieldProps) {
  const controlId = `loop-run-input-${name}`;
  const isBoolean = field.type === "boolean";
  const errorId = `loop-run-field-error-${name}`;
  return (
    <div
      className="flex flex-col gap-1.5"
      data-testid={`loop-run-field-${name}`}
      data-input-type={field.type}
    >
      {isBoolean ? (
        <div className="flex items-center gap-3 rounded-md border border-line-soft bg-canvas-tint px-3 py-2.5">
          <LoopTypedInputControl
            controlId={controlId}
            describedBy={error ? errorId : undefined}
            disabled={disabled}
            field={field}
            invalid={Boolean(error)}
            onChange={onChange}
            testId={`loop-run-switch-${name}`}
            value={value}
          />
          <div className="min-w-0 flex-1">
            <label
              htmlFor={controlId}
              className="flex items-center font-mono text-form-input text-fg-strong"
            >
              {name}
              {field.required ? <RequiredMark /> : null}
            </label>
            {field.description ? (
              <p className="text-form-hint leading-snug text-subtle">{field.description}</p>
            ) : null}
          </div>
          <MonoTag className="ml-auto text-faint">{field.type}</MonoTag>
        </div>
      ) : (
        <>
          <label htmlFor={controlId} className="flex items-center gap-1.5">
            <span className="font-mono text-form-input text-fg-strong">{name}</span>
            {field.required ? <RequiredMark /> : null}
            <MonoTag className="ml-auto text-faint">{field.type}</MonoTag>
          </label>
          <LoopTypedInputControl
            controlId={controlId}
            describedBy={error ? errorId : undefined}
            disabled={disabled}
            field={field}
            invalid={Boolean(error)}
            onChange={onChange}
            testId={`loop-run-field-input-${name}`}
            value={value}
          />
        </>
      )}
      {!isBoolean && field.description ? (
        <p className="text-form-hint leading-snug text-subtle">{field.description}</p>
      ) : null}
      {error ? (
        <p
          className="flex items-center gap-1.5 text-form-hint text-danger"
          data-testid={`loop-run-field-error-${name}`}
          id={errorId}
        >
          <AlertCircle className="size-3" aria-hidden="true" />
          {error}
        </p>
      ) : null}
    </div>
  );
}
