import { useId, useState } from "react";
import { AlertTriangle } from "lucide-react";

import { cn, FieldError, Input, NativeSelect, NativeSelectOption, Switch, Textarea } from "@agh/ui";

import { MonoTag } from "../mono-tag";
import { getAtPath } from "../../lib/loop-editor-draft";
import type {
  FieldPath,
  FieldSpec,
  NumberFieldSpec,
  SelectFieldSpec,
  TextFieldSpec,
} from "../../lib/loop-node-schema";
import type { LoopReferenceSuggestion } from "../../lib/loop-references";
import { LoopEditorCriteria } from "./loop-editor-criteria";
import { LoopEditorWatchEvents } from "./loop-editor-watch-events";
import { LoopReferenceInput } from "./loop-reference-input";

interface LoopEditorFieldProps {
  field: FieldSpec;
  raw: Record<string, unknown>;
  suggestions: readonly LoopReferenceSuggestion[];
  disabled: boolean;
  onChange: (path: FieldPath, value: unknown) => void;
}

function str(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function FieldSpecLabel({ field }: { field: TextFieldSpec | NumberFieldSpec | SelectFieldSpec }) {
  const optional = "optionalLabel" in field ? field.optionalLabel : undefined;
  const required = "required" in field ? field.required : false;
  return (
    <span className="mb-1.5 flex items-center gap-1.5 text-form-label font-medium text-fg-strong">
      {field.label}
      {required ? (
        <span className="text-accent-strong" title="required">
          *
        </span>
      ) : null}
      {optional ? <span className="text-badge font-normal text-faint">{optional}</span> : null}
    </span>
  );
}

/** A structured (object/array) field: edited as JSON text, parsed back on change so a
 *  scalar edit never coerces the field to a string. Invalid JSON is surfaced, not saved. */
function JsonField({
  value,
  disabled,
  onCommit,
  testId,
  ariaLabel,
}: {
  value: unknown;
  disabled: boolean;
  onCommit: (parsed: unknown) => void;
  testId?: string;
  ariaLabel?: string;
}) {
  const [text, setText] = useState(() =>
    value === undefined ? "" : JSON.stringify(value, null, 2)
  );
  const [error, setError] = useState<string | null>(null);
  const errorId = useId();
  return (
    <div>
      <Textarea
        aria-describedby={error ? errorId : undefined}
        aria-invalid={Boolean(error)}
        aria-label={ariaLabel}
        className="min-h-18.5 resize-y text-form-input leading-relaxed"
        data-testid={testId}
        disabled={disabled}
        value={text}
        variant="mono"
        onChange={event => {
          const next = event.target.value;
          setText(next);
          if (next.trim() === "") {
            setError(null);
            onCommit(undefined);
            return;
          }
          try {
            const parsed = JSON.parse(next);
            setError(null);
            onCommit(parsed);
          } catch {
            setError("Invalid JSON — not saved");
          }
        }}
      />
      {error ? (
        <FieldError className="mt-1.5 flex items-center gap-1.5 text-form-hint" id={errorId}>
          <AlertTriangle aria-hidden="true" className="size-3" />
          {error}
        </FieldError>
      ) : null}
    </div>
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
      <JsonField
        value={value}
        disabled={disabled}
        testId={testId}
        ariaLabel={field.label}
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
      <div>
        <span className="mb-1.5 block text-form-label font-medium text-fg-strong">
          {field.label}
        </span>
        <span className="flex items-center gap-2 text-small-body text-fg">
          <span className="font-mono text-mono-id text-fg-strong">{field.value || "—"}</span>
          {field.badge ? (
            <MonoTag className="ml-auto rounded-xs bg-badge-fill px-1.5 py-0.5 text-pill-group-badge text-faint">
              {field.badge}
            </MonoTag>
          ) : null}
        </span>
      </div>
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
    return (
      <div>
        <FieldSpecLabel field={field} />
        <NativeSelect
          value={str(getAtPath(raw, field.path))}
          disabled={disabled}
          onChange={event => onChange(field.path, event.target.value)}
          aria-label={field.label}
          data-testid={`loop-field-${field.key}`}
        >
          {field.options.map(option => (
            <NativeSelectOption key={option} value={option}>
              {option}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        {field.hint ? (
          <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
        ) : null}
      </div>
    );
  }
  if (field.type === "number") {
    const raw0 = getAtPath(raw, field.path);
    const numeric = typeof raw0 === "number" ? raw0 : Number(str(raw0));
    const over = field.ceiling !== undefined && Number.isFinite(numeric) && numeric > field.ceiling;
    return (
      <div>
        <FieldSpecLabel field={field} />
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
            className="mt-1.5 flex items-center gap-1.5 text-form-hint text-danger"
            data-testid={`loop-field-${field.key}-ceiling`}
          >
            <AlertTriangle aria-hidden="true" className="size-3" />
            Above the ceiling of {field.ceiling} — the linter rejects a higher value on publish.
          </p>
        ) : null}
        {field.hint ? (
          <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
        ) : null}
      </div>
    );
  }
  if (field.type === "criteria") {
    return (
      <div>
        <span className="mb-1.5 block text-form-label font-medium text-fg-strong">
          {field.label}
        </span>
        <LoopEditorCriteria
          value={getAtPath(raw, field.path)}
          suggestions={props.suggestions}
          disabled={disabled}
          allowedTypes={field.allowedTypes}
          onChange={criteria => onChange(field.path, criteria)}
        />
        {field.hint ? (
          <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
        ) : null}
      </div>
    );
  }
  if (field.type === "events") {
    return (
      <div>
        <span className="mb-1.5 block text-form-label font-medium text-fg-strong">
          {field.label}
        </span>
        <LoopEditorWatchEvents
          value={getAtPath(raw, field.path)}
          suggestions={props.suggestions}
          disabled={disabled}
          onChange={events => onChange(field.path, events)}
        />
        {field.hint ? (
          <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
        ) : null}
      </div>
    );
  }
  if (field.type === "fold") {
    return (
      <details className="rounded-md border border-line-soft bg-canvas-soft">
        <summary className="flex cursor-pointer items-center gap-2 px-3 py-2.5 text-form-label font-medium text-fg-strong">
          {field.label}
          {field.subLabel ? (
            <span className="text-badge font-normal text-faint">{field.subLabel}</span>
          ) : null}
        </summary>
        <div className="flex flex-col gap-3 px-3 pb-3">
          {field.fields.map(child => (
            <LoopEditorField key={child.key} {...props} field={child} />
          ))}
        </div>
      </details>
    );
  }
  return (
    <div>
      <FieldSpecLabel field={field} />
      <TextControl {...props} field={field} />
      {field.hint ? (
        <p className="mt-1.5 text-form-hint leading-relaxed text-subtle">{field.hint}</p>
      ) : null}
    </div>
  );
}
