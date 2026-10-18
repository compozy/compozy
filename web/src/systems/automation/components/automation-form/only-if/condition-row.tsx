import { X } from "lucide-react";

import { Button, FieldError, Input, NativeSelect, NativeSelectOption } from "@compozy/ui";

import { conditionFieldLabel, conditionFieldName } from "../../../lib/automation-form-events";
import type { ConditionProblem } from "../../../lib/automation-form-readiness";

const PROBLEM_TEXT: Record<ConditionProblem, string> = {
  empty: "Add a value or remove this condition.",
  duplicate: "This field already has a condition. Remove one of them.",
};

interface ConditionRowProps {
  index: number;
  field: string;
  value: string;
  fieldOptions: readonly string[];
  openPayload: boolean;
  datalistId: string;
  problem?: ConditionProblem;
  onFieldChange: (next: string) => void;
  onValueChange: (next: string) => void;
  onRemove: () => void;
}

const MONO = "font-mono text-form-label";

/** One `field = value` exact-match condition. */
export function ConditionRow({
  index,
  field,
  value,
  fieldOptions,
  openPayload,
  datalistId,
  problem,
  onFieldChange,
  onValueChange,
  onRemove,
}: ConditionRowProps) {
  const errorId = `automation-condition-error-${index}`;
  const incomplete = problem !== undefined;
  const valueLabel = field.trim()
    ? `Value for ${conditionFieldName(field.trim())}`
    : "Condition value";
  return (
    <div className="flex flex-col gap-1">
      <div className="grid grid-cols-[minmax(0,1.3fr)_auto_minmax(0,1fr)_auto] items-center gap-2">
        {openPayload ? (
          <Input
            aria-label="Condition field"
            className={MONO}
            data-testid={`automation-condition-field-${index}`}
            list={datalistId}
            onChange={event => onFieldChange(event.target.value)}
            placeholder="data.…"
            value={field}
          />
        ) : (
          <NativeSelect
            aria-label="Condition field"
            className="w-full"
            data-testid={`automation-condition-field-${index}`}
            onChange={event => onFieldChange(event.target.value)}
            value={field}
          >
            {fieldOptions.map(option => (
              <NativeSelectOption key={option} value={option}>
                {conditionFieldLabel(option)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        )}
        <span aria-hidden="true" className="font-mono text-small-body text-subtle">
          =
        </span>
        <Input
          aria-describedby={incomplete ? errorId : undefined}
          aria-invalid={incomplete}
          aria-label={valueLabel}
          className={MONO}
          data-testid={`automation-condition-value-${index}`}
          onChange={event => onValueChange(event.target.value)}
          placeholder="value"
          value={value}
        />
        <Button
          aria-label="Remove condition"
          data-testid={`automation-condition-remove-${index}`}
          onClick={onRemove}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <X aria-hidden="true" />
        </Button>
      </div>
      {problem ? <FieldError id={errorId}>{PROBLEM_TEXT[problem]}</FieldError> : null}
    </div>
  );
}
