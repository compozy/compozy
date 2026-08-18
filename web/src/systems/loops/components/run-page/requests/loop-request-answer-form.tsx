import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Switch,
  Textarea,
} from "@compozy/ui";

import type { LoopRequestField } from "../../../lib/loop-request-payload";

export interface LoopRequestAnswerFormProps {
  idPrefix: string;
  fields: readonly LoopRequestField[];
  values: Readonly<Record<string, string>>;
  errors: Readonly<Record<string, string>>;

  isRaw: boolean;
  rawValue: string;
  disabled?: boolean;
  onChange: (name: string, value: string) => void;
  onRawChange: (value: string) => void;
}

export function LoopRequestAnswerForm({
  idPrefix,
  fields,
  values,
  errors,
  isRaw,
  rawValue,
  disabled,
  onChange,
  onRawChange,
}: LoopRequestAnswerFormProps) {
  if (isRaw) {
    return (
      <RawPayloadField
        disabled={disabled}
        error={errors.payload}
        idPrefix={idPrefix}
        onRawChange={onRawChange}
        value={rawValue}
      />
    );
  }
  if (fields.length === 0) return null;
  return (
    <div className="flex flex-col gap-3.5">
      {fields.map(field => (
        <AnswerField
          disabled={disabled}
          error={errors[field.name]}
          field={field}
          idPrefix={idPrefix}
          key={field.name}
          onChange={onChange}
          value={values[field.name] ?? ""}
        />
      ))}
    </div>
  );
}

interface AnswerFieldProps {
  field: LoopRequestField;
  idPrefix: string;
  value: string;
  error?: string;
  disabled?: boolean;
  onChange: (name: string, value: string) => void;
}

function AnswerField({ field, idPrefix, value, error, disabled, onChange }: AnswerFieldProps) {
  const controlId = `${idPrefix}-${field.name}`;
  const errorId = `${controlId}-error`;
  const describedBy = error ? errorId : undefined;
  return (
    <Field data-invalid={error ? true : undefined}>
      <FieldLabel htmlFor={controlId}>
        {field.name}
        <span className="font-mono text-mono-id text-faint">{typeHint(field)}</span>
      </FieldLabel>
      <AnswerControl
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        field={field}
        invalid={Boolean(error)}
        onChange={onChange}
        value={value}
      />
      {field.description ? <FieldDescription>{field.description}</FieldDescription> : null}
      {error ? (
        <FieldError data-testid={`loop-request-field-error-${field.name}`} id={errorId}>
          {error}
        </FieldError>
      ) : null}
    </Field>
  );
}

interface AnswerControlProps {
  field: LoopRequestField;
  controlId: string;
  value: string;
  invalid: boolean;
  disabled?: boolean;
  describedBy?: string;
  onChange: (name: string, value: string) => void;
}

function AnswerControl({
  field,
  controlId,
  value,
  invalid,
  disabled,
  describedBy,
  onChange,
}: AnswerControlProps) {
  const shared = {
    "aria-describedby": describedBy,
    "aria-invalid": invalid || undefined,
    "data-testid": `loop-request-field-${field.name}`,
    disabled,
    id: controlId,
  };
  if (field.type === "boolean") {
    const checked = value === "true";
    return (
      <span className="flex items-center gap-2.5">
        <Switch
          {...shared}
          checked={checked}
          onCheckedChange={next => onChange(field.name, next ? "true" : "false")}
        />
        <span className="text-form-hint text-muted">{checked ? "true" : "false"}</span>
      </span>
    );
  }
  if (field.options.length > 0) {
    return (
      <NativeSelect
        {...shared}
        className="w-full"
        onChange={event => onChange(field.name, event.target.value)}
        size="sm"
        value={value}
      >
        <NativeSelectOption value="">Not set</NativeSelectOption>
        {field.options.map(option => (
          <NativeSelectOption key={option} value={option}>
            {option}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    );
  }
  if (field.type === "json") {
    return (
      <Textarea
        {...shared}
        className="font-mono text-mono-id"
        onChange={event => onChange(field.name, event.target.value)}
        rows={3}
        value={value}
      />
    );
  }
  const isNumeric = field.type === "number" || field.type === "integer";
  return (
    <Input
      {...shared}
      inputMode={isNumeric ? "decimal" : undefined}
      onChange={event => onChange(field.name, event.target.value)}
      type={isNumeric ? "number" : "text"}
      value={value}
    />
  );
}

function RawPayloadField({
  idPrefix,
  value,
  error,
  disabled,
  onRawChange,
}: {
  idPrefix: string;
  value: string;
  error?: string;
  disabled?: boolean;
  onRawChange: (value: string) => void;
}) {
  const controlId = `${idPrefix}-payload`;
  const errorId = `${controlId}-error`;
  return (
    <Field data-invalid={error ? true : undefined}>
      <FieldLabel htmlFor={controlId}>
        Payload
        <span className="font-mono text-mono-id text-faint">json</span>
      </FieldLabel>
      <Textarea
        aria-describedby={error ? errorId : undefined}
        aria-invalid={error ? true : undefined}
        className="font-mono text-mono-id"
        data-testid="loop-request-raw-payload"
        disabled={disabled}
        id={controlId}
        onChange={event => onRawChange(event.target.value)}
        rows={5}
        value={value}
      />
      {error ? (
        <FieldError data-testid="loop-request-field-error-payload" id={errorId}>
          {error}
        </FieldError>
      ) : null}
    </Field>
  );
}

function typeHint(field: LoopRequestField): string {
  return field.required ? `${field.type} · required` : field.type;
}
