import { pad2, parseCron } from "./cron-engine-parsing";

const DOW_LONG = [
  "Sunday",
  "Monday",
  "Tuesday",
  "Wednesday",
  "Thursday",
  "Friday",
  "Saturday",
] as const;
const DOW_SHORT = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"] as const;
const MONTH_SHORT = [
  "Jan",
  "Feb",
  "Mar",
  "Apr",
  "May",
  "Jun",
  "Jul",
  "Aug",
  "Sep",
  "Oct",
  "Nov",
  "Dec",
] as const;

export function formatClock(hour: number, minute: number): string {
  return `${pad2(hour)}:${pad2(minute)}`;
}

function weekdayPhrase(set: Set<number>): string {
  const days = [...set].sort((left, right) => left - right);
  if (days.length === 5 && days.join(",") === "1,2,3,4,5") return "every weekday";
  if (days.length === 2 && days.join(",") === "0,6") return "weekends";
  if (days.length === 1) return `every ${DOW_LONG[days[0]]}`;
  return `on ${days.map(day => DOW_SHORT[day]).join(", ")}`;
}

/** Plain-language rendering of a cron expression, or `null` for custom shapes. */
export function humanCron(expr: string): string | null {
  const parts = String(expr).trim().split(/\s+/);
  const parsed = parseCron(expr);
  if (!parsed) return null;

  const [minuteField, hourField, domField, monthField, dowField] = parts;
  const token = (field: string): number | null =>
    !field.includes(",") && !field.includes("-") && !field.includes("/") && field !== "*"
      ? Number.parseInt(field, 10)
      : null;
  const minute = token(minuteField);
  const hour = token(hourField);
  const everyMatch = minuteField.match(/^\*\/(\d+)$/);

  if (
    everyMatch &&
    hourField === "*" &&
    domField === "*" &&
    monthField === "*" &&
    dowField === "*"
  ) {
    return `every ${everyMatch[1]} minutes`;
  }
  if ([minuteField, hourField, domField, monthField, dowField].every(field => field === "*")) {
    return "every minute";
  }
  if (
    minute !== null &&
    hourField === "*" &&
    domField === "*" &&
    monthField === "*" &&
    dowField === "*"
  ) {
    return minute === 0 ? "every hour, on the hour" : `every hour at :${pad2(minute)}`;
  }
  if (minute !== null && hour !== null && domField === "*" && monthField === "*") {
    const isMidnight = hour === 0 && minute === 0;
    const time = isMidnight ? "midnight" : `at ${formatClock(hour, minute)}`;
    if (dowField === "*") return isMidnight ? "every day at midnight" : `every day ${time}`;
    return `${weekdayPhrase(parsed.dayOfWeek.set)} ${isMidnight ? "at midnight" : time}`;
  }
  if (
    minute !== null &&
    hour !== null &&
    monthField === "*" &&
    dowField === "*" &&
    domField !== "*"
  ) {
    const day = token(domField);
    if (day !== null) return `on day ${day} of every month at ${formatClock(hour, minute)}`;
  }
  return null;
}

export function formatAbsoluteUtc(date: Date): string {
  return (
    `${DOW_SHORT[date.getUTCDay()]} ${MONTH_SHORT[date.getUTCMonth()]} ${date.getUTCDate()}, ` +
    formatClock(date.getUTCHours(), date.getUTCMinutes())
  );
}

export function formatRelative(date: Date, now: number = Date.now()): string {
  const seconds = Math.round((date.getTime() - now) / 1_000);
  if (seconds < 0) return "now";
  if (seconds < 60) return `in ${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `in ${minutes} min`;
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (hours < 24) return `in ${hours}h${remainingMinutes ? ` ${remainingMinutes}m` : ""}`;
  const days = Math.floor(hours / 24);
  const remainingHours = hours % 24;
  return `in ${days}d${remainingHours ? ` ${remainingHours}h` : ""}`;
}

export const SCHEDULE_CONSTANTS = { DOW_LONG, DOW_SHORT, MONTH_SHORT } as const;
