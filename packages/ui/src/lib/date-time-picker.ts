import { addMinutes, isSameDay, setHours, setMinutes, startOfDay } from "date-fns";

function atTime(day: Date, hours: number, minutes: number): Date {
  return setMinutes(setHours(startOfDay(day), hours), minutes);
}

/** Every `step` minutes of `day`, from midnight. */
export function dateTimePickerSlots(day: Date, step: number): Date[] {
  const safeStep = Math.max(1, Math.floor(step));
  const start = startOfDay(day);
  const slots: Date[] = [];
  for (let offset = 0; offset < 24 * 60; offset += safeStep) {
    const slot = addMinutes(start, offset);
    // A DST jump can move a slot onto the next day; the list stays on `day`.
    if (isSameDay(slot, start)) slots.push(slot);
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
