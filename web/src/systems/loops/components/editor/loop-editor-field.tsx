import { AlertTriangle } from "lucide-react";

import {
  cn,
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  RequiredMark,
  Switch,
  Textarea,
} from "@compozy/ui";

import type { WorktreePayload } from "@/systems/workspace";

import type { RawLoopNode } from "../../lib/codec";
import { getAtPath, type NodeFieldEdit } from "../../lib/loop-editor-draft";
import type {
  FieldPath,
  FieldSpec,
  NumberFieldSpec,
  SelectFieldSpec,
  TextFieldSpec,
} from "../../lib/loop-node-schema";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import type { LoopEnvironmentSpec, LoopValidationIssue } from "../../types";
import { MonoTag } from "../mono-tag";
import { LoopEditorCriteria } from "./loop-editor-criteria";
import { LoopEditorEffectsField } from "./loop-editor-field-effects";
import { LoopEditorEnvironmentField } from "./loop-editor-environment-field";
import { LoopEditorJsonField } from "./loop-editor-json-field";
import { LoopEditorWaitMode } from "./loop-editor-wait-mode";
import { LoopEditorFold } from "./loop-editor-fold";
import { LoopEditorWatchEvents } from "./loop-editor-watch-events";
import { LoopReferenceInput } from "./loop-reference-input";

interface LoopEditorFieldProps {
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

function TextControl({
  field,
  raw,
  suggestions,
  disabled,
  onChange,
}: LoopEditorFieldProps & { field: TextFieldSpec }) {
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

/** Renders one inspector field from its DSL-derived descriptor. */
export function LoopEditorField(props: LoopEditorFieldProps) {
  const { field, raw, disabled, onChange } = props;

  if (field.type === "hint") {
    return <p className="text-form-hint leading-relaxed text-subtle">{field.hint}</p>;
  }
  if (field.type === "static") {
    return (
      <Field>
        <FieldHeader>
          <FieldLabel>{field.label}</FieldLabel>
        </FieldHeader>
        <span className="flex items-center gap-2 text-small-body text-fg">
          <span className="font-mono text-mono-id text-fg-strong">{field.value || "—"}</span>
          {field.badge ? (
            <MonoTag className="ml-auto rounded-xs bg-badge-fill px-1.5 py-0.5 text-pill-group-badge text-faint">
              {field.badge}
            </MonoTag>
          ) : null}
        </span>
      </Field>
    );
  }
  if (field.type === "switch") {
    const checked = Boolean(getAtPath(raw, field.path));
    return (
      <div className="flex items-center gap-3 rounded-md border border-line-soft bg-canvas-soft px-3 py-2.5">
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
  if (field.type === "select") {
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
          {clear ? (
            <NativeSelectOption value={clear.value}>{clear.label}</NativeSelectOption>
          ) : null}
          {field.options.map(option => (
            <NativeSelectOption key={option} value={option}>
              {option}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {field.hint ? <FieldDescription>{field.hint}</FieldDescription> : null}
      </Field>
    );
  }
  if (field.type === "number") {
    const raw0 = getAtPath(raw, field.path);
    const numeric = typeof raw0 === "number" ? raw0 : Number(str(raw0));
    const over = field.ceiling !== undefined && Number.isFinite(numeric) && numeric > field.ceiling;
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
            onChange={event => {
              const next = event.target.value;
              if (next === "") {
                if (event.currentTarget.validity.badInput) return;
                onChange(field.path, undefined);
                return;
              }
              const parsed = Number(next);
              if (Number.isFinite(parsed)) onChange(field.path, parsed);
            }}
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
        {field.hint ? <FieldDescription>{field.hint}</FieldDescription> : null}
      </Field>
    );
  }
  if (field.type === "criteria") {
    return (
      <Field>
        <FieldHeader>
          <FieldLabel>{field.label}</FieldLabel>
        </FieldHeader>
        <LoopEditorCriteria
          value={getAtPath(raw, field.path)}
          suggestions={props.suggestions}
          disabled={disabled}
          allowedTypes={field.allowedTypes}
          onChange={criteria => onChange(field.path, criteria)}
        />
        {field.hint ? <FieldDescription>{field.hint}</FieldDescription> : null}
      </Field>
    );
  }
  if (field.type === "events") {
    return (
      <Field>
        <FieldHeader>
          <FieldLabel>{field.label}</FieldLabel>
        </FieldHeader>
        <LoopEditorWatchEvents
          value={getAtPath(raw, field.path)}
          suggestions={props.suggestions}
          disabled={disabled}
          onChange={events => onChange(field.path, events)}
        />
        {field.hint ? <FieldDescription>{field.hint}</FieldDescription> : null}
      </Field>
    );
  }
  if (field.type === "effects") {
    return (
      <LoopEditorEffectsField
        field={field}
        raw={raw}
        suggestions={props.suggestions}
        disabled={disabled}
        onChange={onChange}
      />
    );
  }
  if (field.type === "wait-mode") {
    return (
      <LoopEditorWaitMode
        field={field}
        raw={raw as RawLoopNode}
        disabled={disabled}
        onChangeFields={props.onChangeFields}
      />
    );
  }
  if (field.type === "environment") {
    return (
      <LoopEditorEnvironmentField
        disabled={disabled}
        field={field}
        gitBacked={props.gitBacked}
        lintIssues={props.lintIssues}
        loopDefaultEnvironment={props.loopDefaultEnvironment}
        onChange={props.onChange}
        onChangeFields={props.onChangeFields}
        raw={raw as RawLoopNode}
        suggestions={props.suggestions}
        worktrees={props.worktrees}
      />
    );
  }
  if (field.type === "fold") {
    return (
      <LoopEditorFold defaultOpen={field.defaultOpen} label={field.label} subLabel={field.subLabel}>
        {field.fields.map(child => (
          <LoopEditorField key={child.key} {...props} field={child} />
        ))}
      </LoopEditorFold>
    );
  }
  return (
    <Field>
      <SpecFieldLabel field={field} />
      <TextControl {...props} field={field} />
      {field.hint ? <FieldDescription>{field.hint}</FieldDescription> : null}
    </Field>
  );
}
