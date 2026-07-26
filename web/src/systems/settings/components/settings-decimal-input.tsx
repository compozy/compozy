import { useState, type ComponentProps } from "react";

import { Input } from "@agh/ui";

interface SettingsDecimalInputProps extends Omit<
  ComponentProps<typeof Input>,
  "onChange" | "type" | "value"
> {
  value: number;
  min?: number;
  max?: number;
  precision?: number;
  onValueChange: (value: number) => void;
  onValidityChange?: (message: string | null) => void;
}

function formatNumber(value: number, precision?: number): string {
  if (!Number.isFinite(value)) {
    return "0";
  }
  if (precision === undefined) {
    return String(value);
  }
  return value.toFixed(precision);
}

function validateDecimalInput(
  rawValue: string,
  min: number | undefined,
  max: number | undefined
): string | null {
  const trimmed = rawValue.trim();
  if (trimmed === "") {
    return "Enter a value.";
  }

  if (!/^-?\d+(\.\d+)?$/.test(trimmed)) {
    return "Enter a number.";
  }

  const parsed = Number.parseFloat(trimmed);
  if (!Number.isFinite(parsed)) {
    return "Enter a number.";
  }

  if (typeof min === "number" && parsed < min) {
    return `Value must be ${min} or greater.`;
  }
  if (typeof max === "number" && parsed > max) {
    return `Value must be ${max} or less.`;
  }

  return null;
}

function SettingsDecimalInput({
  value,
  min,
  max,
  precision,
  onValueChange,
  onValidityChange,
  ...props
}: SettingsDecimalInputProps) {
  const [rawValue, setRawValue] = useState(() => formatNumber(value, precision));

  const validationMessage = validateDecimalInput(rawValue, min, max);

  return (
    <Input
      {...props}
      type="text"
      inputMode="decimal"
      value={rawValue}
      aria-invalid={validationMessage ? true : undefined}
      onChange={event => {
        const nextRawValue = event.target.value;
        setRawValue(nextRawValue);

        const nextValidationMessage = validateDecimalInput(nextRawValue, min, max);
        onValidityChange?.(nextValidationMessage);
        if (nextValidationMessage) {
          return;
        }

        onValueChange(Number.parseFloat(nextRawValue));
      }}
    />
  );
}

export { SettingsDecimalInput };
