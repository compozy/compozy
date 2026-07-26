const MINUTE_MS = 60_000;
const HOUR_MS = 3_600_000;
const DAY_MS = 86_400_000;

export type CronFrequency = "minutes" | "hourly" | "daily" | "weekly" | "monthly" | "custom";

/** Friendly builder model layered over the raw five-field cron expression. */
export interface CronModel {
  frequency: CronFrequency;
  everyMinutes: number;
  hourlyMinute: number;
  hour: number;
  minute: number;
  weekdays: number[];
  monthDay: number;
}

export interface CronField {
  any: boolean;
  set: Set<number>;
}

export interface ParsedCron {
  minute: CronField;
  hour: CronField;
  dayOfMonth: CronField;
  month: CronField;
  dayOfWeek: CronField;
}

const DEFAULT_CRON_MODEL: CronModel = {
  frequency: "daily",
  everyMinutes: 15,
  hourlyMinute: 0,
  hour: 9,
  minute: 0,
  weekdays: [1, 2, 3, 4, 5],
  monthDay: 1,
};

export function fullCronModel(partial?: Partial<CronModel>): CronModel {
  return {
    ...DEFAULT_CRON_MODEL,
    ...partial,
    weekdays: partial?.weekdays ? [...partial.weekdays] : [...DEFAULT_CRON_MODEL.weekdays],
  };
}

export function pad2(value: number): string {
  return String(value).padStart(2, "0");
}

function parseCronField(raw: string, lo: number, hi: number): CronField {
  if (raw === "*") return { any: true, set: new Set() };

  const set = new Set<number>();
  for (const part of raw.split(",")) {
    let rangeText = part;
    let step = 1;
    const slash = part.indexOf("/");
    if (slash >= 0) {
      rangeText = part.slice(0, slash);
      step = Number.parseInt(part.slice(slash + 1), 10);
      if (!Number.isInteger(step) || step < 1) throw new Error("invalid cron step");
    }

    let start: number;
    let end: number;
    if (rangeText === "*") {
      start = lo;
      end = hi;
    } else if (rangeText.includes("-")) {
      const [rangeStart, rangeEnd] = rangeText.split("-");
      start = Number.parseInt(rangeStart, 10);
      end = Number.parseInt(rangeEnd, 10);
    } else {
      start = Number.parseInt(rangeText, 10);
      end = start;
    }
    if (Number.isNaN(start) || Number.isNaN(end) || start < lo || end > hi || start > end) {
      throw new Error("cron field out of range");
    }
    for (let value = start; value <= end; value += step) set.add(value);
  }
  if (set.size === 0) throw new Error("empty cron field");
  return { any: false, set };
}

/** Parse a five-field cron into matchable fields, or `null` when malformed. */
export function parseCron(expr: string): ParsedCron | null {
  const parts = String(expr).trim().split(/\s+/);
  if (parts.length !== 5) return null;
  try {
    return {
      minute: parseCronField(parts[0], 0, 59),
      hour: parseCronField(parts[1], 0, 23),
      dayOfMonth: parseCronField(parts[2], 1, 31),
      month: parseCronField(parts[3], 1, 12),
      dayOfWeek: parseCronField(parts[4], 0, 6),
    };
  } catch {
    return null;
  }
}

export function isValidCron(expr: string): boolean {
  return parseCron(expr) !== null;
}

function singleValue(field: string): number | null {
  return /^\d+$/.test(field) ? Number.parseInt(field, 10) : null;
}

/** Reverse a cron string into the friendly builder model, or `null` for custom shapes. */
export function decodeCron(expr: string): Partial<CronModel> | null {
  const parts = String(expr).trim().split(/\s+/);
  if (parts.length !== 5 || !parseCron(expr)) return null;
  const [minuteField, hourField, domField, monthField, dowField] = parts;
  const everyMatch = minuteField.match(/^\*\/(\d+)$/);

  if (
    everyMatch &&
    hourField === "*" &&
    domField === "*" &&
    monthField === "*" &&
    dowField === "*"
  ) {
    return { frequency: "minutes", everyMinutes: Number.parseInt(everyMatch[1], 10) };
  }

  const minute = singleValue(minuteField);
  const hour = singleValue(hourField);
  const monthDay = singleValue(domField);
  if (
    minute !== null &&
    hourField === "*" &&
    domField === "*" &&
    monthField === "*" &&
    dowField === "*"
  ) {
    return { frequency: "hourly", hourlyMinute: minute };
  }
  if (
    minute !== null &&
    hour !== null &&
    domField === "*" &&
    monthField === "*" &&
    dowField === "*"
  ) {
    return { frequency: "daily", hour, minute };
  }
  if (
    minute !== null &&
    hour !== null &&
    domField === "*" &&
    monthField === "*" &&
    dowField !== "*"
  ) {
    try {
      const field = parseCronField(dowField, 0, 6);
      const weekdays = Array.from({ length: 7 }, (_, day) => day).filter(day => field.set.has(day));
      return weekdays.length > 0 ? { frequency: "weekly", hour, minute, weekdays } : null;
    } catch {
      return null;
    }
  }
  if (
    minute !== null &&
    hour !== null &&
    monthDay !== null &&
    monthField === "*" &&
    dowField === "*"
  ) {
    return { frequency: "monthly", hour, minute, monthDay };
  }
  return null;
}

/** Collapse a weekday list to the most compact cron day-of-week token. */
export function compressWeekdays(days: number[]): string {
  const sorted = [...new Set(days)].sort((left, right) => left - right);
  if (sorted.length >= 7) return "*";
  const contiguous = sorted.every((day, index) => index === 0 || day === sorted[index - 1] + 1);
  if (contiguous && sorted.length >= 3) return `${sorted[0]}-${sorted[sorted.length - 1]}`;
  return sorted.join(",");
}

/** Project the builder model to a five-field cron string. */
export function compileCron(model: CronModel): string | null {
  switch (model.frequency) {
    case "minutes":
      return `*/${model.everyMinutes} * * * *`;
    case "hourly":
      return `${model.hourlyMinute} * * * *`;
    case "daily":
      return `${model.minute} ${model.hour} * * *`;
    case "weekly":
      return `${model.minute} ${model.hour} * * ${compressWeekdays(model.weekdays)}`;
    case "monthly":
      return `${model.minute} ${model.hour} ${model.monthDay} * *`;
    case "custom":
      return null;
  }
}

/** Parse a positive Go-style duration (`30m`, `1h`, `2h30m`, `45s`) to milliseconds. */
export function parseDuration(value: string): number | null {
  const text = String(value).trim();
  if (!/^(\d+(h|m|s))+$/.test(text)) return null;
  let milliseconds = 0;
  const segment = /(\d+)(h|m|s)/g;
  let match: RegExpExecArray | null;
  while ((match = segment.exec(text)) !== null) {
    const amount = Number.parseInt(match[1], 10);
    milliseconds +=
      match[2] === "h" ? amount * HOUR_MS : match[2] === "m" ? amount * MINUTE_MS : amount * 1_000;
  }
  return milliseconds > 0 ? milliseconds : null;
}

/** Convert a timezone-naive `datetime-local` value to its UTC wall-clock `Date`. */
export function localInputToDate(value: string): Date | null {
  const match = String(value).match(/^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/);
  if (!match) return null;
  return new Date(
    Date.UTC(
      Number(match[1]),
      Number(match[2]) - 1,
      Number(match[3]),
      Number(match[4]),
      Number(match[5]),
      0
    )
  );
}

/** Render a `Date` as the timezone-naive `datetime-local` string in UTC. */
export function dateToLocalInput(date: Date): string {
  return (
    `${date.getUTCFullYear()}-${pad2(date.getUTCMonth() + 1)}-${pad2(date.getUTCDate())}` +
    `T${pad2(date.getUTCHours())}:${pad2(date.getUTCMinutes())}`
  );
}

export function toRfc3339(date: Date | null): string {
  return date ? date.toISOString().replace(/\.\d{3}Z$/, "Z") : "";
}

export function defaultAtLocal(now: number = Date.now()): string {
  const date = new Date(now + DAY_MS);
  date.setUTCMinutes(0, 0, 0);
  return dateToLocalInput(date);
}
