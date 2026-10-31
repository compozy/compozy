import * as React from "react";

import { dateTimePickerDayValue, dateTimePickerPresetValue } from "../../../lib/date-time-picker";

interface DateTimePickerPresetValue {
  value: Date | (() => Date);
}

interface UseDateTimePickerOptions {
  value: Date | null;
  onValueChange: (value: Date | null) => void;
  min?: Date;
  minuteStep: number;
  defaultTime: readonly [number, number];
  disabled?: boolean;
}

/**
 * Popover, visible month, and pick handlers of `DateTimePicker`. Opening
 * returns the calendar to the value's month; the chosen time scrolls into view.
 */
export function useDateTimePicker({
  value,
  onValueChange,
  min,
  minuteStep,
  defaultTime,
  disabled = false,
}: UseDateTimePickerOptions) {
  const [requestedOpen, setOpen] = React.useState(false);
  // A disabled picker shows no popover, so nothing in it can change the value.
  const open = requestedOpen && !disabled;
  const [month, setMonth] = React.useState<Date>(() => value ?? min ?? new Date());
  const selectedSlotRef = React.useRef<HTMLButtonElement | null>(null);

  React.useEffect(() => {
    if (!open) return;
    const frame = requestAnimationFrame(() => {
      selectedSlotRef.current?.scrollIntoView?.({ block: "center" });
    });
    return () => cancelAnimationFrame(frame);
  }, [open, value]);

  return {
    open,
    month,
    setMonth,
    selectedSlotRef,
    openChange: (next: boolean) => {
      if (next && disabled) return;
      if (next) setMonth(value ?? min ?? new Date());
      setOpen(next);
    },
    pickDay: (picked: Date | undefined) => {
      if (!picked || disabled) return;
      const next = dateTimePickerDayValue(picked, value, { min, step: minuteStep, defaultTime });
      if (next) onValueChange(next);
    },
    pickTime: (slot: Date) => {
      if (disabled || (min && slot.getTime() < min.getTime())) return;
      onValueChange(slot);
      setOpen(false);
    },
    pickPreset: (preset: DateTimePickerPresetValue) => {
      const next = dateTimePickerPresetValue(preset.value, min);
      if (disabled || next === null) return;
      onValueChange(next);
      setMonth(next);
      setOpen(false);
    },
  };
}
