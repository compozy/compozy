"use client";

import { startOfDay } from "date-fns";
import { CalendarClock, X } from "lucide-react";
import * as React from "react";

import {
  dateTimePickerPresetValue,
  dateTimePickerSlotLabels,
  dateTimePickerSlots,
} from "../../lib/date-time-picker";
import { cn } from "../../lib/utils";
import { Button } from "../button";
import { Calendar } from "../calendar";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";
import { ScrollArea } from "../scroll-area";
import { useDateTimePicker } from "./hooks/use-date-time-picker";

/** A one-click choice beside the calendar; `value` resolves when it is picked. */
export interface DateTimePickerPreset {
  label: string;
  value: Date | (() => Date);
}

export interface DateTimePickerProps {
  /** The chosen instant, or `null` while nothing is chosen. */
  value: Date | null;
  onValueChange: (value: Date | null) => void;
  /** The earliest selectable instant; earlier days and times are disabled. */
  min?: Date;
  presets?: readonly DateTimePickerPreset[];
  /** Spacing of the time list, in minutes. */
  minuteStep?: number;
  /** Time of day a freshly picked day starts at, as `[hours, minutes]`. */
  defaultTime?: readonly [number, number];
  placeholder?: string;
  /** Accessible name of the clear button; the clear button shows only with a value. */
  clearLabel?: string;
  timeLabel?: string;
  presetsLabel?: string;
  formatValue?: (value: Date) => string;
  id?: string;
  disabled?: boolean;
  className?: string;
  "aria-invalid"?: boolean;
  "aria-describedby"?: string;
  "aria-labelledby"?: string;
  "data-testid"?: string;
}

const VALUE_FORMATTER = new Intl.DateTimeFormat(undefined, {
  weekday: "short",
  month: "short",
  day: "numeric",
  hour: "2-digit",
  minute: "2-digit",
});

/**
 * Date and time in one popover: optional presets, a month calendar, and a
 * time list. Picking a day keeps the popover open for the time; picking a
 * time or a preset closes it. The value stays clearable from the trigger row.
 */
export function DateTimePicker({
  value,
  onValueChange,
  min,
  presets,
  minuteStep = 30,
  defaultTime = [9, 0],
  placeholder = "Pick a date",
  clearLabel = "Clear date",
  timeLabel = "Time",
  presetsLabel = "Suggestions",
  formatValue = date => VALUE_FORMATTER.format(date),
  id,
  disabled,
  className,
  "aria-invalid": ariaInvalid,
  "aria-describedby": ariaDescribedBy,
  "aria-labelledby": ariaLabelledBy,
  "data-testid": testId,
}: DateTimePickerProps) {
  const generatedId = React.useId();
  const triggerId = id ?? generatedId;
  const { open, month, setMonth, selectedSlotRef, openChange, pickDay, pickTime, pickPreset } =
    useDateTimePicker({ value, onValueChange, min, minuteStep, defaultTime, disabled });
  const slots = value ? dateTimePickerSlots(value, minuteStep, value) : [];
  const formatters = React.useMemo(
    () => ({
      time: new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit" }),
      zoned: new Intl.DateTimeFormat(undefined, {
        hour: "2-digit",
        minute: "2-digit",
        timeZoneName: "short",
      }),
    }),
    []
  );
  const slotLabels = dateTimePickerSlotLabels(slots, formatters);

  return (
    <div
      className={cn("flex min-w-0 items-center gap-1", className)}
      data-slot="date-time-picker"
      data-testid={testId}
    >
      <Popover open={open} onOpenChange={openChange}>
        <PopoverTrigger
          render={
            <Button
              aria-describedby={ariaDescribedBy}
              aria-invalid={ariaInvalid}
              // The field label names the trigger and its own text reads the value.
              aria-labelledby={ariaLabelledBy ? `${ariaLabelledBy} ${triggerId}` : undefined}
              className="min-w-0 justify-start rounded-md border-line bg-canvas px-3 font-normal hover:border-line-strong hover:bg-canvas"
              data-slot="date-time-picker-trigger"
              disabled={disabled}
              id={triggerId}
              type="button"
              variant="outline"
            />
          }
        >
          <CalendarClock aria-hidden="true" className="text-subtle" />
          <span className={cn("truncate", value ? "text-fg" : "text-subtle")}>
            {value ? formatValue(value) : placeholder}
          </span>
        </PopoverTrigger>
        <PopoverContent align="start" className="w-auto p-0">
          <div className="flex max-sm:flex-col">
            {presets && presets.length > 0 ? (
              <div
                aria-label={presetsLabel}
                className="flex flex-col gap-0.5 border-line-soft p-2 max-sm:order-1 max-sm:border-t sm:w-36 sm:border-e"
                role="group"
              >
                {presets.map(preset => (
                  <Button
                    className="w-full justify-start font-normal"
                    data-slot="date-time-picker-preset"
                    // A preset that resolves before `min` stays visible but closed.
                    disabled={dateTimePickerPresetValue(preset.value, min) === null}
                    key={preset.label}
                    onClick={() => pickPreset(preset)}
                    size="sm"
                    type="button"
                    variant="quiet"
                  >
                    {preset.label}
                  </Button>
                ))}
              </div>
            ) : null}
            <Calendar
              disabled={min ? { before: startOfDay(min) } : undefined}
              mode="single"
              month={month}
              onMonthChange={setMonth}
              onSelect={pickDay}
              selected={value ?? undefined}
            />
            <div className="relative h-56 border-line-soft max-sm:border-t sm:h-auto sm:w-28 sm:border-s">
              {/* Base UI pins the scroll root to position: relative, so an inner frame
                  takes the column's height and the calendar alone sizes the row. */}
              <div className="absolute inset-0">
                <ScrollArea className="h-full" viewportClassName="p-2">
                  {value ? (
                    <div aria-label={timeLabel} className="flex flex-col gap-0.5" role="group">
                      {slots.map((slot, index) => {
                        const selected = value !== null && slot.getTime() === value.getTime();
                        const closed = min !== undefined && slot.getTime() < min.getTime();
                        return (
                          <Button
                            aria-pressed={selected}
                            className="w-full justify-center font-normal tabular-nums aria-pressed:bg-primary aria-pressed:text-primary-foreground"
                            data-slot="date-time-picker-time"
                            disabled={closed}
                            key={slot.getTime()}
                            onClick={() => pickTime(slot)}
                            ref={selected ? selectedSlotRef : undefined}
                            size="sm"
                            type="button"
                            variant="quiet"
                          >
                            {slotLabels[index]}
                          </Button>
                        );
                      })}
                    </div>
                  ) : (
                    <p className="px-1 py-2 text-center text-eyebrow text-subtle">
                      Pick a day first
                    </p>
                  )}
                </ScrollArea>
              </div>
            </div>
          </div>
        </PopoverContent>
      </Popover>
      {value && !disabled ? (
        <Button
          aria-label={clearLabel}
          data-slot="date-time-picker-clear"
          onClick={() => onValueChange(null)}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          <X aria-hidden="true" />
        </Button>
      ) : null}
    </div>
  );
}
