import { addDays, addMinutes, isSameDay } from "date-fns";

function atTime(day: Date, hours: number, minutes: number): Date {
  return new Date(day.getFullYear(), day.getMonth(), day.getDate(), hours, minutes);
}

/**
 * The time list of `day`: every `step` minutes from its midnight to the next
 * one. Walking elapsed time up to the next local midnight keeps a clock-change
 * day complete: a 25-hour day lists both instants of its repeated hour and
 * still ends at 23:30, and a 23-hour day skips the hour that does not exist.
 * `include` (the chosen value) joins the list when it falls between slots.
 */
export function dateTimePickerSlots(day: Date, step: number, include?: Date | null): Date[] {
  const safeStep = Math.max(1, Math.floor(step));
  const start = atTime(day, 0, 0);
  const end = atTime(addDays(start, 1), 0, 0);
  const slots: Date[] = [];
  for (let slot = start; slot < end; slot = addMinutes(slot, safeStep)) slots.push(slot);
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
  if (!Number.isFinite(resolved.getTime())) return null;
  return min && resolved.getTime() < min.getTime() ? null : resolved;
}

/** Formatters for the time list; built where the current zone is known. */
export interface DateTimePickerSlotFormatters {
  time: Intl.DateTimeFormat;
  /** The same time with the zone's short name, for a repeated hour. */
  zoned: Intl.DateTimeFormat;
}

/**
 * The label of each slot. The two instants of a repeated hour (the day clocks
 * go back) would read the same, so those carry their zone name.
 */
export function dateTimePickerSlotLabels(
  slots: readonly Date[],
  formatters: DateTimePickerSlotFormatters
): string[] {
  const labels = slots.map(slot => formatters.time.format(slot));
  const counts = new Map<string, number>();
  for (const label of labels) counts.set(label, (counts.get(label) ?? 0) + 1);
  return labels.map((label, index) =>
    (counts.get(label) ?? 0) > 1 ? formatters.zoned.format(slots[index]) : label
  );
}
