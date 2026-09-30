import { useId } from "react";

import { Field, FieldLabel, Label, RadioGroup, RadioGroupItem } from "@compozy/ui";

import type { SessionDerivePlacement } from "../hooks/use-session-derive";

const PLACEMENTS: ReadonlyArray<{ value: SessionDerivePlacement; label: string }> = [
  { value: "new-window", label: "New window" },
  { value: "this-window", label: "This window" },
];

export interface SessionDerivePlacementFieldProps {
  value: SessionDerivePlacement;
  onChange: (next: SessionDerivePlacement) => void;
  disabled?: boolean;
}

/** "Open in" for the Continue and Fork dialogs; New window is the default. */
export function SessionDerivePlacementField({
  value,
  onChange,
  disabled = false,
}: SessionDerivePlacementFieldProps) {
  const id = useId();
  const labelId = `${id}-label`;
  return (
    <Field>
      <FieldLabel id={labelId}>Open in</FieldLabel>
      <RadioGroup
        aria-labelledby={labelId}
        className="flex flex-wrap gap-x-4.5 gap-y-2"
        data-testid="session-derive-placement"
        disabled={disabled}
        onValueChange={next => onChange(next as SessionDerivePlacement)}
        value={value}
      >
        {PLACEMENTS.map(option => (
          <div className="flex items-center gap-2" key={option.value}>
            <RadioGroupItem
              data-testid={`session-derive-placement-${option.value}`}
              id={`${id}-${option.value}`}
              value={option.value}
            />
            <Label
              className="text-form-input font-normal text-fg"
              htmlFor={`${id}-${option.value}`}
            >
              {option.label}
            </Label>
          </div>
        ))}
      </RadioGroup>
    </Field>
  );
}
