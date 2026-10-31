import * as React from "react";

import { dateTimePickerDayValue } from "../../../lib/date-time-picker";

interface DateTimePickerPresetValue {
  value: Date | (() => Date);
}

interface UseDateTimePickerOptions {
  value: Date | null;
  onValueChange: (value: Date | null) => void;
  min?: Date;
  minuteStep: number;
  defaultTime: readonly [number, number];
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
}: UseDateTimePickerOptions) {
  const [open, setOpen] = React.useState(false);
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
      if (next) setMonth(value ?? min ?? new Date());
      setOpen(next);
    },
    pickDay: (picked: Date | undefined) => {
      if (!picked) return;
      const next = dateTimePickerDayValue(picked, value, { min, step: minuteStep, defaultTime });
      if (next) onValueChange(next);
    },
    pickTime: (slot: Date) => {
      onValueChange(slot);
      setOpen(false);
    },
    pickPreset: (preset: DateTimePickerPresetValue) => {
      const next = typeof preset.value === "function" ? preset.value() : preset.value;
      onValueChange(next);
      setMonth(next);
      setOpen(false);
    },
  };
}
