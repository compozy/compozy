import { X } from "lucide-react";

import { Button, Input, NativeSelect, NativeSelectOption } from "@agh/ui";

interface FilterRowProps {
  index: number;
  conditionKey: string;
  value: string;
  keyOptions: string[];
  openPayload: boolean;
  datalistId: string;
  onKeyChange: (next: string) => void;
  onValueChange: (next: string) => void;
  onRemove: () => void;
}

const MONO_INPUT = "font-mono text-form-label";

/** One `key = value` condition row. */
export function FilterRow({
  index,
  conditionKey,
  value,
  keyOptions,
  openPayload,
  datalistId,
  onKeyChange,
  onValueChange,
  onRemove,
}: FilterRowProps) {
  return (
    <div className="grid grid-cols-[1.25fr_auto_1fr_auto] items-center gap-2">
      {openPayload ? (
        <Input
          aria-label="Condition field"
          className={MONO_INPUT}
          data-testid={`trigger-filter-key-${index}`}
          list={datalistId}
          onChange={event => onKeyChange(event.target.value)}
          placeholder="kind · scope · data.…"
          value={conditionKey}
        />
      ) : (
        <NativeSelect
          aria-label="Condition field"
          className={`w-full ${MONO_INPUT}`}
          data-testid={`trigger-filter-key-${index}`}
          onChange={event => onKeyChange(event.target.value)}
          value={conditionKey}
        >
          {keyOptions.map(option => (
            <NativeSelectOption key={option} value={option}>
              {option}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      )}
      <span className="text-center font-mono text-small-body text-subtle">=</span>
      <Input
        aria-label="Condition value"
        className={MONO_INPUT}
        data-testid={`trigger-filter-value-${index}`}
        onChange={event => onValueChange(event.target.value)}
        placeholder="value"
        value={value}
      />
      <Button
        aria-label="Remove condition"
        data-testid={`trigger-filter-remove-${index}`}
        onClick={onRemove}
        size="icon-sm"
        type="button"
        variant="ghost"
      >
        <X aria-hidden="true" />
      </Button>
    </div>
  );
}
