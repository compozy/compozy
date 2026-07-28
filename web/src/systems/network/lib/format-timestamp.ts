const clockFormatter = new Intl.DateTimeFormat("en-US", {
  hour: "numeric",
  minute: "2-digit",
});

const clockSecondsFormatter = new Intl.DateTimeFormat("en-US", {
  hour: "numeric",
  minute: "2-digit",
  second: "2-digit",
});

const monthDayFormatter = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
});

const weekdayFormatter = new Intl.DateTimeFormat("en-US", { weekday: "long" });

const isoToleranceMs = 1_000;

export interface DatePillReference {
  /** Reference "now" used to compute TODAY / YESTERDAY relative buckets. */
  now?: Date;
}

function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

function differenceInCalendarDays(target: Date, reference: Date): number {
  const utcTarget = Date.UTC(target.getFullYear(), target.getMonth(), target.getDate());
  const utcReference = Date.UTC(reference.getFullYear(), reference.getMonth(), reference.getDate());
  return Math.floor((utcReference - utcTarget) / 86_400_000);
}

function parseTimestamp(value: string | Date | null | undefined): Date | null {
  if (value == null) {
    return null;
  }
  const parsed = value instanceof Date ? value : new Date(value);
  return Number.isNaN(parsed.getTime()) ? null : parsed;
}

export function formatTimelineClock(value: string | Date | null | undefined): string {
  const parsed = parseTimestamp(value);
  if (!parsed) {
    return "";
  }
  return clockFormatter.format(parsed);
}

export function formatTimelineClockWithSeconds(value: string | Date | null | undefined): string {
  const parsed = parseTimestamp(value);
  if (!parsed) {
    return "";
  }
  return clockSecondsFormatter.format(parsed);
}

export function formatTimelineIso(value: string | Date | null | undefined): string {
  const parsed = parseTimestamp(value);
  if (!parsed) {
    return "";
  }
  return parsed.toISOString();
}

export function formatDatePill(
  value: string | Date | null | undefined,
  options: DatePillReference = {}
): string {
  const target = parseTimestamp(value);
  if (!target) {
    return "";
  }
  const reference = options.now ?? new Date();
  const dayDelta = differenceInCalendarDays(target, reference);

  if (dayDelta === 0) {
    return "TODAY";
  }
  if (dayDelta === 1) {
    return "YESTERDAY";
  }

  const crossesYear = target.getFullYear() !== reference.getFullYear();
  if (crossesYear) {
    return `${target.getFullYear()} · ${monthDayFormatter.format(target).toUpperCase()}`;
  }

  if (dayDelta > 1 && dayDelta < 7) {
    return weekdayFormatter.format(target).toUpperCase();
  }

  return `${weekdayFormatter.format(target).toUpperCase()} · ${monthDayFormatter.format(target).toUpperCase()}`;
}

export function isWithinSeconds(
  current: string | Date | null | undefined,
  previous: string | Date | null | undefined,
  windowSeconds: number
): boolean {
  const currentDate = parseTimestamp(current);
  const previousDate = parseTimestamp(previous);
  if (!currentDate || !previousDate) {
    return false;
  }
  const deltaMs = currentDate.getTime() - previousDate.getTime();
  if (deltaMs < -isoToleranceMs) {
    return false;
  }
  return deltaMs <= windowSeconds * 1_000;
}

export function isSameCalendarDay(
  current: string | Date | null | undefined,
  previous: string | Date | null | undefined
): boolean {
  const currentDate = parseTimestamp(current);
  const previousDate = parseTimestamp(previous);
  if (!currentDate || !previousDate) {
    return false;
  }
  return isSameDay(currentDate, previousDate);
}

export const TIMELINE_GROUP_WINDOW_SECONDS = 60;
