import { useState } from "react";

import { Input, NativeSelect, NativeSelectOption } from "@agh/ui";

export type SettingsByteUnit = "GB" | "MB" | "KB";

const UNIT_FACTOR: Record<SettingsByteUnit, number> = {
  GB: 1024 ** 3,
  MB: 1024 ** 2,
  KB: 1024,
};

const UNITS: readonly SettingsByteUnit[] = ["GB", "MB", "KB"];

function preferredUnit(bytes: number): SettingsByteUnit {
  for (const unit of UNITS) {
    if (bytes > 0 && bytes % UNIT_FACTOR[unit] === 0) return unit;
  }
  return "MB";
}

function displayValue(bytes: number, unit: SettingsByteUnit): string {
  if (bytes <= 0) return "";
  const scaled = bytes / UNIT_FACTOR[unit];
  return Number.isInteger(scaled) ? String(scaled) : scaled.toFixed(2);
}

export interface SettingsByteFieldProps {
  /** Canonical value in bytes (what the config stores). */
  value: number;
  onChange: (bytes: number) => void;
  /** Accessible label, e.g. "Global storage cap". */
  label: string;
  disabled?: boolean;
  "data-testid"?: string;
}

/**
 * Byte-size input (prototype `.bytefield`): number + unit select with a live
 * byte equivalence so nobody types 1073741824 by hand.
 */
export function SettingsByteField({
  value,
  onChange,
  label,
  disabled = false,
  "data-testid": testId,
}: SettingsByteFieldProps) {
  const [unit, setUnit] = useState<SettingsByteUnit>(() => preferredUnit(value));
  const [draft, setDraft] = useState<string>(() => displayValue(value, preferredUnit(value)));

  const commit = (nextDraft: string, nextUnit: SettingsByteUnit) => {
    const scaled = Number.parseFloat(nextDraft);
    if (!Number.isFinite(scaled) || scaled < 0) return;
    onChange(Math.round(scaled * UNIT_FACTOR[nextUnit]));
  };

  return (
    <div className="flex flex-col items-end gap-1" data-testid={testId ?? "settings-byte-field"}>
      <div className="flex items-center gap-1.5">
        <Input
          aria-label={label}
          className="h-7 w-24 text-right font-mono text-form-input tabular-nums"
          disabled={disabled}
          inputMode="decimal"
          onChange={event => {
            setDraft(event.target.value);
            commit(event.target.value, unit);
          }}
          value={draft}
        />
        <NativeSelect
          aria-label={`${label} unit`}
          className="w-20"
          disabled={disabled}
          onChange={event => {
            const nextUnit = event.target.value as SettingsByteUnit;
            setUnit(nextUnit);
            commit(draft, nextUnit);
          }}
          size="sm"
          value={unit}
        >
          {UNITS.map(option => (
            <NativeSelectOption key={option} value={option}>
              {option}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>
      <span
        className="font-mono text-mono-id tabular-nums text-faint"
        data-slot="settings-byte-equivalence"
      >
        = {value.toLocaleString()} bytes
      </span>
    </div>
  );
}
