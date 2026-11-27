import { useId, useState, type ReactNode } from "react";

import {
  Field,
  FieldDescription,
  FieldError,
  FieldHeader,
  FieldLabel,
  Input,
  Switch,
} from "@compozy/ui";

import type { ExtensionInputDefinitions } from "./extension-install-model";
import type { ExtensionInputForm } from "./use-extension-input-form";

type ExtensionInputDefinition = ExtensionInputDefinitions[number];

/** Per-field setup state: a marker beside the label and an optional line under it. */
interface ExtensionInputAnnotation {
  marker?: ReactNode;
  description?: ReactNode;
}

interface ExtensionInputFieldsProps {
  /** Freezes every control while the install/update request is in flight. */
  disabled?: boolean;
  form: ExtensionInputForm;
  /** Replaces the "optional" marker, e.g. with an installed input's state. */
  annotate?: (input: ExtensionInputDefinition) => ExtensionInputAnnotation | undefined;
  /** Daemon-reported field errors; shown without waiting for interaction. */
  fieldErrors?: Partial<Record<string, string>>;
}

/** Field errors wait for interaction; the model gates submission immediately. */
function ExtensionInputFields({
  annotate,
  disabled = false,
  fieldErrors,
  form,
}: ExtensionInputFieldsProps) {
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
        const error =
          fieldErrors?.[input.id] ?? (touched.has(input.id) ? form.errors[input.id] : undefined);
        const raw = form.draft[input.id];
        const shared = {
          annotation: annotate?.(input),
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
  annotation: ExtensionInputAnnotation | undefined;
  controlId: string;
  disabled: boolean;
  error: string | undefined;
  errorId: string;
  input: ExtensionInputDefinition;
  onTouch: () => void;
}

function ExtensionFieldLabel({
  annotation,
  controlId,
  input,
}: Pick<ExtensionFieldProps, "annotation" | "controlId" | "input">) {
  return (
    <FieldHeader>
      <FieldLabel htmlFor={controlId}>{input.prompt}</FieldLabel>
      {annotation ? (
        annotation.marker
      ) : input.required ? null : (
        <span className="text-form-hint whitespace-nowrap text-faint">· optional</span>
      )}
    </FieldHeader>
  );
}

function ExtensionFieldDescription({
  annotation,
  id,
}: {
  annotation: ExtensionInputAnnotation | undefined;
  id: string;
}) {
  return annotation?.description ? (
    <FieldDescription id={id}>{annotation.description}</FieldDescription>
  ) : null;
}

function describedBy(
  annotation: ExtensionInputAnnotation | undefined,
  descriptionId: string,
  error: string | undefined,
  errorId: string
): string | undefined {
  const ids = [annotation?.description ? descriptionId : "", error ? errorId : ""].filter(Boolean);
  return ids.length ? ids.join(" ") : undefined;
}

function ExtensionTextField({
  annotation,
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
      <ExtensionFieldLabel annotation={annotation} controlId={controlId} input={input} />
      <ExtensionFieldDescription annotation={annotation} id={`${controlId}-description`} />
      <Input
        aria-describedby={describedBy(annotation, `${controlId}-description`, error, errorId)}
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
  annotation,
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
        <ExtensionFieldLabel annotation={annotation} controlId={controlId} input={input} />
        <Switch
          aria-describedby={describedBy(annotation, `${controlId}-description`, error, errorId)}
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
      <ExtensionFieldDescription annotation={annotation} id={`${controlId}-description`} />
      {error ? (
        <FieldError data-testid={`extension-input-${input.id}-error`} id={errorId}>
          {error}
        </FieldError>
      ) : null}
    </Field>
  );
}

export { ExtensionInputFields };
export type { ExtensionInputAnnotation, ExtensionInputFieldsProps };
