import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  RadioCard,
  RequiredMark,
  Textarea,
} from "@compozy/ui";

import { type LoopRequestField, loopRequestFieldLabel } from "../../../lib/loop-request-payload";
import {
  LoopEntityListValueControl,
  LoopEntityValueControl,
} from "../../input/loop-typed-input-control";

export interface LoopRequestAnswerFormProps {
  idPrefix: string;
  /** Element id of the question prompt; labels a lone field so it is not named twice. */
  promptId: string;
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
  promptId,
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
  const lone = fields.length === 1;
  return (
    <div className="flex flex-col gap-3.5">
      {fields.map(field => (
        <AnswerField
          disabled={disabled}
          error={errors[field.name]}
          field={field}
          idPrefix={idPrefix}
          key={field.name}
          labelledBy={lone && field.control.kind !== "entity" ? promptId : undefined}
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
  /** When set, the question prompt names this field and no label renders. */
  labelledBy?: string;
  onChange: (name: string, value: string) => void;
}

interface AnswerFieldLayout {
  controlId: string;
  labelId: string;
  errorId: string;
  describedBy?: string;
  /** Choice lists are named by aria-labelledby, so their label points at no control. */
  labelFor?: string;
  controlLabelledBy?: string;
  placeholder?: string;
  showDescription: boolean;
}

/** Derives ids and label/description placement for one answer field. */
function answerFieldLayout(
  field: LoopRequestField,
  idPrefix: string,
  error: string | undefined,
  labelledBy: string | undefined
): AnswerFieldLayout {
  const controlId = `${idPrefix}-${field.name}`;
  const labelId = `${controlId}-label`;
  const errorId = `${controlId}-error`;
  const isChoice = field.control.kind === "select" || field.control.kind === "boolean";
  const asPlaceholder = labelledBy !== undefined && !isChoice;
  const hasDescription = field.description !== "";
  return {
    controlId,
    labelId,
    errorId,
    describedBy: error ? errorId : undefined,
    labelFor: isChoice ? undefined : controlId,
    controlLabelledBy: labelledBy ?? (isChoice ? labelId : undefined),
    placeholder: asPlaceholder && hasDescription ? field.description : undefined,
    showDescription: hasDescription && !asPlaceholder,
  };
}

interface AnswerFieldLabelProps {
  field: LoopRequestField;
  labelId: string;
  htmlFor?: string;
}

function AnswerFieldLabel({ field, labelId, htmlFor }: AnswerFieldLabelProps) {
  return (
    <FieldLabel htmlFor={htmlFor} id={labelId}>
      {loopRequestFieldLabel(field)}
      {field.required ? <RequiredMark className="ml-0" /> : null}
      {field.control.kind === "json" ? (
        <span className="font-mono text-mono-id text-faint">JSON</span>
      ) : null}
    </FieldLabel>
  );
}

function AnswerField({
  field,
  idPrefix,
  value,
  error,
  disabled,
  labelledBy,
  onChange,
}: AnswerFieldProps) {
  const layout = answerFieldLayout(field, idPrefix, error, labelledBy);
  return (
    <Field data-invalid={error ? true : undefined}>
      {labelledBy === undefined ? (
        <AnswerFieldLabel field={field} htmlFor={layout.labelFor} labelId={layout.labelId} />
      ) : null}
      <AnswerControl
        controlId={layout.controlId}
        describedBy={layout.describedBy}
        disabled={disabled}
        field={field}
        invalid={Boolean(error)}
        labelledBy={layout.controlLabelledBy}
        lone={labelledBy !== undefined}
        onChange={onChange}
        placeholder={layout.placeholder}
        value={value}
      />
      {layout.showDescription ? <FieldDescription>{field.description}</FieldDescription> : null}
      {error ? (
        <FieldError data-testid={`loop-request-field-error-${field.name}`} id={layout.errorId}>
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
  lone: boolean;
  disabled?: boolean;
  describedBy?: string;
  labelledBy?: string;
  placeholder?: string;
  onChange: (name: string, value: string) => void;
}

interface ChoiceOption {
  token: string;
  label: string;
}

const BOOLEAN_OPTIONS: readonly ChoiceOption[] = [
  { token: "true", label: "Yes" },
  { token: "false", label: "No" },
];

function AnswerControl({
  field,
  controlId,
  value,
  invalid,
  lone,
  disabled,
  describedBy,
  labelledBy,
  placeholder,
  onChange,
}: AnswerControlProps) {
  if (field.control.kind === "select" || field.control.kind === "boolean") {
    return (
      <LoopRequestChoiceList
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        labelledBy={labelledBy}
        name={field.name}
        onChange={onChange}
        options={field.control.kind === "select" ? field.control.options : BOOLEAN_OPTIONS}
        value={value}
      />
    );
  }
  if (field.control.kind === "entity") {
    return (
      <LoopEntityValueControl
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        kind={field.control.entityKind}
        onChange={next => onChange(field.name, next)}
        testId={`loop-request-field-${field.name}`}
        value={value}
      />
    );
  }
  if (field.control.kind === "entity-list") {
    return (
      <LoopEntityListValueControl
        controlId={controlId}
        describedBy={describedBy}
        disabled={disabled}
        invalid={invalid}
        kind={field.control.entityKind}
        onChange={next => onChange(field.name, next)}
        testId={`loop-request-field-${field.name}`}
        value={value}
      />
    );
  }
  const shared = {
    "aria-describedby": describedBy,
    "aria-invalid": invalid || undefined,
    "aria-labelledby": labelledBy,
    "data-testid": `loop-request-field-${field.name}`,
    disabled,
    id: controlId,
    placeholder,
    required: field.required,
  };
  if (field.control.kind === "json") {
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
  if (field.control.kind === "number" || field.control.kind === "integer") {
    return (
      <Input
        {...shared}
        inputMode="decimal"
        onChange={event => onChange(field.name, event.target.value)}
        type="number"
        value={value}
      />
    );
  }
  if (lone) {
    return (
      <Textarea
        {...shared}
        onChange={event => onChange(field.name, event.target.value)}
        rows={2}
        value={value}
      />
    );
  }
  return (
    <Input
      {...shared}
      onChange={event => onChange(field.name, event.target.value)}
      type="text"
      value={value}
    />
  );
}

function LoopRequestChoiceList({
  name,
  options,
  value,
  invalid,
  disabled,
  describedBy,
  labelledBy,
  onChange,
}: {
  name: string;
  options: readonly ChoiceOption[];
  value: string;
  invalid: boolean;
  disabled?: boolean;
  describedBy?: string;
  labelledBy?: string;
  onChange: (name: string, value: string) => void;
}) {
  return (
    <div
      aria-describedby={describedBy}
      aria-invalid={invalid || undefined}
      aria-labelledby={labelledBy}
      className="flex flex-col gap-1.5"
      data-testid={`loop-request-field-${name}`}
      role="radiogroup"
    >
      {options.map(option => (
        <RadioCard
          data-testid={`loop-request-option-${name}-${option.token}`}
          disabled={disabled}
          key={option.token}
          onSelect={() => onChange(name, option.token)}
          selected={value === option.token}
          title={option.label}
        />
      ))}
    </div>
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
        Answer
        <span className="font-mono text-mono-id text-faint">JSON</span>
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
