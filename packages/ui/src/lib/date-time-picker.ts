import { isSameDay } from "date-fns";

function atTime(day: Date, hours: number, minutes: number): Date {
  return new Date(day.getFullYear(), day.getMonth(), day.getDate(), hours, minutes);
}

/**
 * The time list of `day`: every `step` minutes of its wall clock, from 00:00
 * to the last slot before midnight. Walking the wall clock instead of elapsed
 * time keeps a 23- or 25-hour day (a clock change) complete; a wall-clock
 * time the clock skips is left out. `include` (the chosen value) joins the
 * list when it falls between slots on this day.
 */
export function dateTimePickerSlots(day: Date, step: number, include?: Date | null): Date[] {
  const safeStep = Math.max(1, Math.floor(step));
  const slots: Date[] = [];
  for (let offset = 0; offset < 24 * 60; offset += safeStep) {
    const hours = Math.floor(offset / 60);
    const minutes = offset % 60;
    const slot = atTime(day, hours, minutes);
    if (slot.getHours() === hours && slot.getMinutes() === minutes) slots.push(slot);
  }
  if (
    include &&
    isSameDay(include, day) &&
    !slots.some(slot => slot.getTime() === include.getTime())
  ) {
    slots.push(include);
    slots.sort((a, b) => a.getTime() - b.getTime());
  }
  return slots;
}

/**
 * The instant a picked day lands on: the current time of day when there is
 * one, otherwise `defaultTime`, moved to the first open slot when `min`
 * closes it. `null` when the whole day is closed.
 */
export function dateTimePickerDayValue(
  day: Date,
  current: Date | null,
  options: { min?: Date; step: number; defaultTime: readonly [number, number] }
): Date | null {
  const [hours, minutes] = current
    ? [current.getHours(), current.getMinutes()]
    : options.defaultTime;
  const candidate = atTime(day, hours, minutes);
  if (!options.min || candidate.getTime() >= options.min.getTime()) return candidate;
  const min = options.min;
  return dateTimePickerSlots(day, options.step).find(slot => slot >= min) ?? null;
}

/** A preset's instant, resolved now; `null` when `min` closes it. */
export function dateTimePickerPresetValue(value: Date | (() => Date), min?: Date): Date | null {
  const resolved = typeof value === "function" ? value() : value;
  return min && resolved.getTime() < min.getTime() ? null : resolved;
}
