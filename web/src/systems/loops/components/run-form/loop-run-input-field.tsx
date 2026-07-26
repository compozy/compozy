import { AlertCircle } from "lucide-react";

import { Input, Switch } from "@agh/ui";

import type { LoopInputSchemaField } from "../../types";
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

function placeholderFor(field: LoopInputSchemaField): string | undefined {
  if (field.type === "agent") return "agent profile";
  if (field.type === "file") return "path or glob";
  if (field.type === "ref") return field.ref?.kind ? `${field.ref.kind} ref` : "resource ref";
  if (field.default === undefined || field.default === null) return undefined;
  if (typeof field.default === "string") return field.default === "" ? undefined : field.default;
  return String(field.default);
}

/** Two-character avatar seed for an `agent` input (matches the design's picker prefix). */
function agentAvatar(value: unknown, field: LoopInputSchemaField): string {
  const raw =
    typeof value === "string" && value.trim() !== "" ? value : String(field.default ?? "");
  return raw.slice(0, 2).toLowerCase() || "ag";
}

/**
 * One auto-generated run-form field, rendered entirely from a declared input's type
 * (§4.3): boolean -> switch, number -> numeric input, agent -> avatar-prefixed picker,
 * file/ref/string -> mono text. Every field carries a type badge, a required marker,
 * an optional hint, and an inline required error. `data-input-type` makes the chosen
 * control type assertable (web-unit-3). The daemon's declared type string is rendered
 * verbatim in the badge (truthful UI — `boolean`, not `bool`).
 */
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
  const isNumber = field.type === "number";
  const isAgent = field.type === "agent";
  return (
    <div
      className="flex flex-col gap-1.5"
      data-testid={`loop-run-field-${name}`}
      data-input-type={field.type}
    >
      {isBoolean ? (
        <div className="flex items-center gap-3 rounded-md border border-line-soft bg-canvas-tint px-3 py-2.5">
          <Switch
            id={controlId}
            data-testid={`loop-run-switch-${name}`}
            checked={typeof value === "boolean" ? value : Boolean(field.default)}
            disabled={disabled}
            onCheckedChange={checked => onChange(checked)}
          />
          <div className="min-w-0 flex-1">
            <label htmlFor={controlId} className="block text-xs font-medium text-fg-strong">
              {name}
            </label>
            {field.description ? (
              <p className="text-form-hint leading-snug text-subtle">{field.description}</p>
            ) : null}
          </div>
          <MonoTag className="rounded-xs bg-badge-fill px-1.5 py-0.5">{field.type}</MonoTag>
        </div>
      ) : (
        <>
          <label htmlFor={controlId} className="flex items-center gap-1.5">
            <span className="text-xs font-medium text-fg-strong">{name}</span>
            {field.required ? (
              <span className="text-form-required font-semibold text-accent" aria-label="required">
                *
              </span>
            ) : null}
            <MonoTag className="ml-auto rounded-xs bg-badge-fill px-1.5 py-0.5">
              {field.type}
            </MonoTag>
          </label>
          <div className="relative">
            {isAgent ? (
              <span
                aria-hidden="true"
                className="absolute top-1/2 left-2 grid size-5 -translate-y-1/2 place-items-center rounded-xs bg-accent-strong font-mono text-pill-group-badge font-semibold text-accent-ink"
              >
                {agentAvatar(value, field)}
              </span>
            ) : null}
            <Input
              id={controlId}
              data-testid={`loop-run-field-input-${name}`}
              type={isNumber ? "number" : "text"}
              className={`font-mono ${isAgent ? "pl-9" : ""} ${error ? "border-danger" : ""}`}
              disabled={disabled}
              placeholder={placeholderFor(field)}
              value={
                isNumber
                  ? typeof value === "number"
                    ? String(value)
                    : ""
                  : typeof value === "string"
                    ? value
                    : ""
              }
              onChange={event => {
                if (!isNumber) {
                  onChange(event.target.value);
                  return;
                }
                const next = event.target.value;
                if (next === "") {
                  onChange(undefined);
                  return;
                }
                // Ignore partial/invalid entry (e.g. "1e") so `NaN` never reaches the run.
                const parsed = Number(next);
                if (!Number.isNaN(parsed)) onChange(parsed);
              }}
            />
          </div>
        </>
      )}
      {!isBoolean && field.description ? (
        <p className="text-form-hint leading-snug text-subtle">{field.description}</p>
      ) : null}
      {error ? (
        <p
          className="flex items-center gap-1.5 text-form-hint text-danger"
          data-testid={`loop-run-field-error-${name}`}
        >
          <AlertCircle className="size-3" aria-hidden="true" />
          {error}
        </p>
      ) : null}
    </div>
  );
}
