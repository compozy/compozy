import { Input } from "@compozy/ui";

import { SettingsNumberInput } from "./settings-number-input";

export function OptionalLimitInput({
  ariaLabel,
  testId,
  value,
  onValueChange,
  onValidityChange,
}: {
  ariaLabel: string;
  testId: string;
  value: number | undefined;
  onValueChange: (value: number) => void;
  onValidityChange: (message: string | null) => void;
}) {
  if (value === undefined) {
    return (
      <Input
        aria-invalid
        aria-label={ariaLabel}
        className="w-28 text-right font-mono"
        data-testid={testId}
        inputMode="numeric"
        onChange={event => {
          const parsed = Number.parseInt(event.target.value, 10);
          if (Number.isSafeInteger(parsed) && parsed >= 1) {
            onValidityChange(null);
            onValueChange(parsed);
          }
        }}
        value=""
      />
    );
  }
  return (
    <SettingsNumberInput
      aria-label={ariaLabel}
      className="w-28 text-right font-mono"
      data-testid={testId}
      min={1}
      onValidityChange={onValidityChange}
      onValueChange={onValueChange}
      value={value}
    />
  );
}
