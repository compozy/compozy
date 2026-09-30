import { AlertTriangle } from "lucide-react";
import type { ChangeEvent } from "react";

import {
  cn,
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  Pill,
  RequiredMark,
  Switch,
  Textarea,
} from "@compozy/ui";

import type { WorktreePayload } from "@/systems/workspace";

import { getAtPath, type NodeFieldEdit } from "../../lib/loop-editor-draft";
import type {
  CriteriaFieldSpec,
  EventsFieldSpec,
  FieldPath,
  FieldSpec,
  HintFieldSpec,
  NumberFieldSpec,
  SelectFieldSpec,
  StaticFieldSpec,
  SwitchFieldSpec,
  TextFieldSpec,
} from "../../lib/loop-node-schema-types";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import type { LoopEnvironmentSpec, LoopValidationIssue } from "../../types";
import { LoopEditorCriteria } from "./loop-editor-criteria";
import { LoopEditorJsonField } from "./loop-editor-json-field";
import { LoopEditorWatchEvents } from "./loop-editor-watch-events";
import { LoopReferenceInput } from "./loop-reference-input";

export interface LoopEditorFieldProps {
  field: FieldSpec;
  raw: Record<string, unknown>;
  suggestions: readonly LoopReferenceSuggestion[];
  disabled: boolean;
  onChange: (path: FieldPath, value: unknown) => void;
  /** Multi-key atomic edits — wait and environment discriminators. */
  onChangeFields: (edits: NodeFieldEdit[]) => void;
  worktrees?: readonly WorktreePayload[];
  gitBacked?: boolean;
  loopDefaultEnvironment?: LoopEnvironmentSpec;
  lintIssues?: readonly LoopValidationIssue[];
}

/** Field props narrowed to one descriptor variant. */
export type LoopEditorFieldControlProps<F extends FieldSpec> = LoopEditorFieldProps & { field: F };

function str(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function SpecFieldLabel({ field }: { field: TextFieldSpec | NumberFieldSpec | SelectFieldSpec }) {
  const optional = "optionalLabel" in field ? field.optionalLabel : undefined;
  const required = "required" in field ? field.required : false;
  return (
    <FieldHeader>
      <FieldLabel>{field.label}</FieldLabel>
      {required ? <RequiredMark>*</RequiredMark> : null}
      {optional ? <span className="text-badge font-normal text-faint">{optional}</span> : null}
    </FieldHeader>
  );
}

function FieldHint({ hint }: { hint?: string }) {
  return hint ? <FieldDescription>{hint}</FieldDescription> : null;
}

function PlainFieldLabel({ label }: { label: string }) {
  return (
    <FieldHeader>
      <FieldLabel>{label}</FieldLabel>
    </FieldHeader>
  );
}

function TextInputControl({
  field,
  raw,
  suggestions,
  disabled,
  onChange,
}: LoopEditorFieldControlProps<TextFieldSpec>) {
  const value = getAtPath(raw, field.path);
  const testId = `loop-field-${field.key}`;
  if (field.json) {
    return (
      <LoopEditorJsonField
        value={value}
        disabled={disabled}
        testId={testId}
        ariaLabel={field.label}
        placeholder={field.placeholder}
        onCommit={parsed => onChange(field.path, parsed)}
      />
    );
  }
  if (field.reference) {
    return (
      <LoopReferenceInput
        value={str(value)}
        onChange={next => onChange(field.path, next)}
        suggestions={suggestions}
        disabled={disabled}
        multiline={field.type === "textarea"}
        mono={field.mono}
        cel={field.cel}
        placeholder={field.placeholder}
        ariaLabel={field.label}
        testId={testId}
      />
    );
  }
  if (field.type === "textarea") {
    return (
      <Textarea
        className={cn(
          "min-h-18.5 resize-y text-form-input leading-relaxed",
          field.mono && "font-mono"
        )}
        value={str(value)}
        disabled={disabled}
        placeholder={field.placeholder}
        aria-label={field.label}
        data-testid={testId}
        onChange={event => onChange(field.path, event.target.value)}
        variant={field.mono ? "mono" : "default"}
      />
    );
  }
  return (
    <Input
      type="text"
      className={cn("h-8 px-2.5 text-form-input", field.mono && "font-mono")}
      value={str(value)}
      disabled={disabled}
      placeholder={field.placeholder}
      aria-label={field.label}
      data-testid={testId}
      onChange={event => onChange(field.path, event.target.value)}
    />
  );
}

export function LoopEditorTextField(props: LoopEditorFieldControlProps<TextFieldSpec>) {
  return (
    <Field>
      <SpecFieldLabel field={props.field} />
      <TextInputControl {...props} />
      <FieldHint hint={props.field.hint} />
    </Field>
  );
}

export function LoopEditorHintField({ field }: LoopEditorFieldControlProps<HintFieldSpec>) {
  return <p className="text-form-hint leading-relaxed text-subtle">{field.hint}</p>;
}

export function LoopEditorStaticField({ field }: LoopEditorFieldControlProps<StaticFieldSpec>) {
  return (
    <Field>
      <PlainFieldLabel label={field.label} />
      <span className="flex items-center gap-2 text-small-body text-fg">
        <span className="font-mono text-mono-id text-fg-strong">{field.value || "—"}</span>
        {field.badge ? (
          <Pill size="xs" tone="neutral" mono className="ml-auto">
            {field.badge}
          </Pill>
        ) : null}
      </span>
    </Field>
  );
}

export function LoopEditorSwitchField({
  field,
  raw,
  disabled,
  onChange,
}: LoopEditorFieldControlProps<SwitchFieldSpec>) {
  const checked = Boolean(getAtPath(raw, field.path));
  return (
    <div className="flex items-center gap-3 rounded-md bg-sunken px-3 py-2.5">
      <Switch
        checked={checked}
        disabled={disabled}
        onCheckedChange={next => onChange(field.path, next)}
        aria-label={field.label}
        data-testid={`loop-field-${field.key}`}
      />
      <span className="min-w-0">
        <span className="block text-form-label font-medium text-fg-strong">{field.label}</span>
        {field.subLabel ? (
          <span className="text-form-hint text-subtle">{field.subLabel}</span>
        ) : null}
      </span>
    </div>
  );
}

export function LoopEditorSelectField({
  field,
  raw,
  disabled,
  onChange,
}: LoopEditorFieldControlProps<SelectFieldSpec>) {
  const clear = field.clearOption;
  return (
    <Field>
      <SpecFieldLabel field={field} />
      <NativeSelect
        value={str(getAtPath(raw, field.path))}
        disabled={disabled}
        onChange={event => {
          const next = event.target.value;
          // An optional DSL enum has no authored default, so selecting the clear option
          // removes the key instead of pinning a value the runtime never received.
          onChange(field.path, clear && next === clear.value ? undefined : next);
        }}
        aria-label={field.label}
        data-testid={`loop-field-${field.key}`}
      >
        {clear ? <NativeSelectOption value={clear.value}>{clear.label}</NativeSelectOption> : null}
        {field.options.map(option => (
          <NativeSelectOption key={option} value={option}>
            {option}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      <FieldHint hint={field.hint} />
    </Field>
  );
}

function isOverCeiling(value: unknown, ceiling: number | undefined): boolean {
  if (ceiling === undefined) return false;
  const numeric = typeof value === "number" ? value : Number(str(value));
  return Number.isFinite(numeric) && numeric > ceiling;
}

export function LoopEditorNumberField({
  field,
  raw,
  disabled,
  onChange,
}: LoopEditorFieldControlProps<NumberFieldSpec>) {
  const raw0 = getAtPath(raw, field.path);
  const over = isOverCeiling(raw0, field.ceiling);
  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    const next = event.target.value;
    if (next === "") {
      if (event.currentTarget.validity.badInput) return;
      onChange(field.path, undefined);
      return;
    }
    const parsed = Number(next);
    if (Number.isFinite(parsed)) onChange(field.path, parsed);
  };
  return (
    <Field>
      <SpecFieldLabel field={field} />
      <div className="flex items-center gap-2.5">
        <Input
          type="number"
          aria-invalid={over || undefined}
          className="h-8 w-28 font-mono text-form-input"
          value={raw0 === undefined || raw0 === null ? "" : String(raw0)}
          disabled={disabled}
          aria-label={field.label}
          data-testid={`loop-field-${field.key}`}
          onChange={handleChange}
        />
        {field.ceiling !== undefined ? (
          <span className="font-mono text-mono-id text-faint">ceiling {field.ceiling}</span>
        ) : null}
      </div>
      {over ? (
        <p
          className="flex items-center gap-1.5 text-form-hint text-danger"
          data-testid={`loop-field-${field.key}-ceiling`}
        >
          <AlertTriangle aria-hidden="true" className="size-3" />
          Above the ceiling of {field.ceiling} — the linter rejects a higher value on publish.
        </p>
      ) : null}
      <FieldHint hint={field.hint} />
    </Field>
  );
}

export function LoopEditorCriteriaField({
  field,
  raw,
  suggestions,
  disabled,
  onChange,
}: LoopEditorFieldControlProps<CriteriaFieldSpec>) {
  return (
    <Field>
      <PlainFieldLabel label={field.label} />
      <LoopEditorCriteria
        value={getAtPath(raw, field.path)}
        suggestions={suggestions}
        disabled={disabled}
        allowedTypes={field.allowedTypes}
        onChange={criteria => onChange(field.path, criteria)}
      />
      <FieldHint hint={field.hint} />
    </Field>
  );
}

export function LoopEditorEventsField({
  field,
  raw,
  suggestions,
  disabled,
  onChange,
}: LoopEditorFieldControlProps<EventsFieldSpec>) {
  return (
    <Field>
      <PlainFieldLabel label={field.label} />
      <LoopEditorWatchEvents
        value={getAtPath(raw, field.path)}
        suggestions={suggestions}
        disabled={disabled}
        onChange={events => onChange(field.path, events)}
      />
      <FieldHint hint={field.hint} />
    </Field>
  );
}
