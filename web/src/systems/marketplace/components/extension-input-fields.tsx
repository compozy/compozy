import { useId, useState } from "react";

import { Field, FieldError, FieldHeader, FieldLabel, Input, Switch } from "@compozy/ui";

import type { ExtensionInputDefinitions } from "./extension-install-model";
import type { ExtensionInputForm } from "./use-extension-input-form";

type ExtensionInputDefinition = ExtensionInputDefinitions[number];

interface ExtensionInputFieldsProps {
  /** Freezes every control while the install/update request is in flight. */
  disabled?: boolean;
  form: ExtensionInputForm;
}

/** Field errors wait for interaction; the model gates submission immediately. */
function ExtensionInputFields({ disabled = false, form }: ExtensionInputFieldsProps) {
  const uid = useId();
  const [touched, setTouched] = useState<ReadonlySet<string>>(() => new Set());
  if (form.definitions.length === 0) return null;

  const touch = (id: string) => {
    setTouched(current => (current.has(id) ? current : new Set(current).add(id)));
  };

  return (
    <div className="flex flex-col gap-4" data-testid="extension-install-inputs">
      {form.definitions.map(input => {
        const controlId = `${uid}-${input.id}`;
        const errorId = `${controlId}-error`;
        const error = touched.has(input.id) ? form.errors[input.id] : undefined;
        const raw = form.draft[input.id];
        const shared = {
          controlId,
          disabled,
          error,
          errorId,
          input,
          onTouch: () => touch(input.id),
        };
        return input.type === "boolean" ? (
          <ExtensionBooleanField
            {...shared}
            checked={raw === true}
            key={input.id}
            onChange={checked => form.change(input.id, checked)}
          />
        ) : (
          <ExtensionTextField
            {...shared}
            key={input.id}
            onChange={value => form.change(input.id, value)}
            value={typeof raw === "string" ? raw : ""}
          />
        );
      })}
    </div>
  );
}

interface ExtensionFieldProps {
  controlId: string;
  disabled: boolean;
  error: string | undefined;
  errorId: string;
  input: ExtensionInputDefinition;
  onTouch: () => void;
}

function ExtensionFieldLabel({
  controlId,
  input,
}: Pick<ExtensionFieldProps, "controlId" | "input">) {
  return (
    <FieldHeader>
      <FieldLabel htmlFor={controlId}>{input.prompt}</FieldLabel>
      {input.required ? null : (
        <span className="text-form-hint whitespace-nowrap text-faint">· optional</span>
      )}
    </FieldHeader>
  );
}

function ExtensionTextField({
  controlId,
  disabled,
  error,
  errorId,
  input,
  onChange,
  onTouch,
  value,
}: ExtensionFieldProps & { value: string; onChange: (value: string) => void }) {
  const secret = input.type === "secret";
  return (
    <Field data-disabled={disabled || undefined} data-invalid={error ? true : undefined}>
      <ExtensionFieldLabel controlId={controlId} input={input} />
      <Input
        aria-describedby={error ? errorId : undefined}
        aria-invalid={error ? true : undefined}
        aria-required={input.required || undefined}
        autoComplete={secret ? "new-password" : "off"}
        className={secret || input.type === "identifier" ? "font-mono" : undefined}
        data-testid={`extension-input-${input.id}`}
        disabled={disabled}
        id={controlId}
        onBlur={onTouch}
        onChange={event => {
          onTouch();
          onChange(event.target.value);
        }}
        spellCheck={secret || input.type === "identifier" ? false : undefined}
        type={secret ? "password" : "text"}
        value={value}
      />
      {error ? (
        <FieldError data-testid={`extension-input-${input.id}-error`} id={errorId}>
          {error}
        </FieldError>
      ) : null}
    </Field>
  );
}

function ExtensionBooleanField({
  checked,
  controlId,
  disabled,
  error,
  errorId,
  input,
  onChange,
  onTouch,
}: ExtensionFieldProps & { checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <Field data-disabled={disabled || undefined} data-invalid={error ? true : undefined}>
      <div className="flex items-center justify-between gap-3">
        <ExtensionFieldLabel controlId={controlId} input={input} />
        <Switch
          aria-describedby={error ? errorId : undefined}
          aria-invalid={error ? true : undefined}
          checked={checked}
          data-testid={`extension-input-${input.id}`}
          disabled={disabled}
          id={controlId}
          onCheckedChange={next => {
            onTouch();
            onChange(next);
          }}
        />
      </div>
      {error ? (
        <FieldError data-testid={`extension-input-${input.id}-error`} id={errorId}>
          {error}
        </FieldError>
      ) : null}
    </Field>
  );
}

export { ExtensionInputFields };
export type { ExtensionInputFieldsProps };
