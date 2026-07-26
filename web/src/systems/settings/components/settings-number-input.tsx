import { useState, type ComponentProps } from "react";

import { Input } from "@agh/ui";

interface SettingsNumberInputProps extends Omit<
  ComponentProps<typeof Input>,
  "onChange" | "type" | "value"
> {
  value: number;
  min?: number;
  resetRevision?: number;
  onValueChange: (value: number) => void;
  onValidityChange?: (message: string | null) => void;
}

function validateIntegerInput(rawValue: string, min: number): string | null {
  const trimmed = rawValue.trim();
  if (trimmed === "") {
    return "Enter a value.";
  }

  const integerPattern = min < 0 ? /^-?\d+$/ : /^\d+$/;
  if (!integerPattern.test(trimmed)) {
    return "Enter a whole number.";
  }

  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isSafeInteger(parsed)) {
    return "Enter a safely representable whole number.";
  }
  if (parsed < min) {
    return `Value must be ${min} or greater.`;
  }

  return null;
}

function SettingsNumberInput({ value, ...props }: SettingsNumberInputProps) {
  return <SettingsNumberInputControl value={value} {...props} />;
}

function SettingsNumberInputControl({
  value,
  min = 0,
  resetRevision = 0,
  onValueChange,
  onValidityChange,
  ...props
}: SettingsNumberInputProps) {
  const [rawValue, setRawValue] = useState(() => String(value));
  const [previous, setPrevious] = useState({ value, resetRevision });
  if (value !== previous.value || resetRevision !== previous.resetRevision) {
    setPrevious({ value, resetRevision });
    setRawValue(String(value));
  }

  const validationMessage = validateIntegerInput(rawValue, min);

  return (
    <Input
      {...props}
      type="text"
      inputMode="numeric"
      pattern={min < 0 ? "-?[0-9]*" : "[0-9]*"}
      value={rawValue}
      aria-invalid={validationMessage ? true : undefined}
      onChange={event => {
        const nextRawValue = event.target.value;
        setRawValue(nextRawValue);

        const nextValidationMessage = validateIntegerInput(nextRawValue, min);
        onValidityChange?.(nextValidationMessage);
        if (nextValidationMessage) {
          return;
        }

        onValueChange(Number.parseInt(nextRawValue, 10));
      }}
    />
  );
}

export { SettingsNumberInput };
