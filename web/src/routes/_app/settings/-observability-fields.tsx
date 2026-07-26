import { Eyebrow } from "@agh/ui";

import { SettingsNumberInput } from "@/systems/settings";

interface NumberFieldProps {
  label: string;
  testId: string;
  value: number;
  suffix?: string;
  errorMessage?: string;
  /** Skip the eyebrow when the surrounding setting row already labels the field. */
  hideLabel?: boolean;
  onValidityChange: (message: string | null) => void;
  onChange: (value: number) => void;
}

export function ObservabilityNumberField({
  label,
  testId,
  value,
  suffix,
  errorMessage,
  hideLabel = false,
  onValidityChange,
  onChange,
}: NumberFieldProps) {
  return (
    <div className="flex flex-col gap-1">
      {hideLabel ? null : <Eyebrow className="text-muted">{label}</Eyebrow>}
      <div className="flex items-center gap-2">
        <SettingsNumberInput
          aria-label={hideLabel ? label : undefined}
          className="w-24"
          min={0}
          data-testid={testId}
          value={value}
          onValidityChange={onValidityChange}
          onValueChange={onChange}
        />
        {suffix ? <span className="text-form-label text-subtle">{suffix}</span> : null}
      </div>
      {errorMessage && !hideLabel ? (
        <span className="text-xs text-danger">{errorMessage}</span>
      ) : null}
    </div>
  );
}
