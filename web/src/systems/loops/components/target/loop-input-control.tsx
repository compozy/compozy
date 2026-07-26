import { Input, Switch } from "@agh/ui";

import type { LoopInputSchemaField } from "../../types";
import { MonoTag } from "../mono-tag";

interface LoopInputControlProps {
  name: string;
  field: LoopInputSchemaField;
  value: unknown;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}

function defaultPlaceholder(field: LoopInputSchemaField): string | undefined {
  if (field.default === undefined || field.default === null) return undefined;
  if (typeof field.default === "string") return field.default === "" ? undefined : field.default;
  return String(field.default);
}

/**
 * One control for a declared Loop input in the automation Target step, chosen by
 * the declared type: boolean -> switch, number -> numeric input, everything else
 * -> text input. Full agent/ref/file pickers ship with the run form (task 20).
 */
export function LoopInputControl({
  name,
  field,
  value,
  disabled,
  onChange,
}: LoopInputControlProps) {
  const controlId = `loop-input-${name}`;
  return (
    <div className="flex flex-col gap-1.5" data-testid="loop-input-control" data-input={name}>
      <label htmlFor={controlId} className="flex items-center gap-1.5">
        <span className="font-mono text-xs text-fg-strong">{name}</span>
        {field.required ? (
          <span className="font-semibold text-muted" aria-label="required">
            *
          </span>
        ) : null}
        <MonoTag className="ml-auto rounded-xs bg-badge-fill px-1.5 py-0.5">{field.type}</MonoTag>
      </label>
      {field.type === "boolean" ? (
        <Switch
          id={controlId}
          data-testid={`loop-input-switch-${name}`}
          checked={typeof value === "boolean" ? value : Boolean(field.default)}
          disabled={disabled}
          onCheckedChange={checked => onChange(checked)}
        />
      ) : field.type === "number" ? (
        <Input
          id={controlId}
          type="number"
          data-testid={`loop-input-field-${name}`}
          className="font-mono"
          disabled={disabled}
          placeholder={defaultPlaceholder(field)}
          value={typeof value === "number" ? String(value) : ""}
          onChange={event => {
            const next = event.target.value;
            if (next === "") {
              onChange(undefined);
              return;
            }
            // Ignore partial/invalid entry (e.g. "1e") so `NaN` never reaches the target.
            const parsed = Number(next);
            if (!Number.isNaN(parsed)) onChange(parsed);
          }}
        />
      ) : (
        <Input
          id={controlId}
          data-testid={`loop-input-field-${name}`}
          className="font-mono"
          disabled={disabled}
          placeholder={defaultPlaceholder(field)}
          value={typeof value === "string" ? value : ""}
          onChange={event => onChange(event.target.value)}
        />
      )}
      {field.description ? (
        <p className="text-form-hint leading-snug text-subtle">{field.description}</p>
      ) : null}
    </div>
  );
}
