import type { LoopInputSchemaField } from "../../types";
import { Pill } from "@compozy/ui";
import { LoopTypedInputControl } from "../input/loop-typed-input-control";

interface LoopInputControlProps {
  name: string;
  field: LoopInputSchemaField;
  value: unknown;
  disabled?: boolean;
  onChange: (value: unknown) => void;
}

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
        <Pill size="xs" tone="neutral" mono className="ml-auto">
          {field.type}
        </Pill>
      </label>
      <LoopTypedInputControl
        controlId={controlId}
        disabled={disabled}
        field={field}
        onChange={onChange}
        testId={field.type === "boolean" ? `loop-input-switch-${name}` : `loop-input-field-${name}`}
        value={value}
      />
      {field.description ? (
        <p className="text-form-hint leading-snug text-subtle">{field.description}</p>
      ) : null}
    </div>
  );
}
