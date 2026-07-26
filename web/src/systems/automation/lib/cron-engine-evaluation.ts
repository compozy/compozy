import { parseCron, type CronField } from "./cron-engine-parsing";

const MINUTE_MS = 60_000;
/** Upper bound on the minute-by-minute scan for the next fire (approximately 367 days). */
const CRON_SCAN_CAP = 367 * 24 * 60;

function matches(field: CronField, value: number): boolean {
  return field.any || field.set.has(value);
}

/** Compute upcoming UTC fire times using Vixie/robfig day matching semantics. */
export function cronNext(expr: string, count: number, now: number = Date.now()): Date[] | null {
  const parsed = parseCron(expr);
  if (!parsed) return null;

  const upcoming: Date[] = [];
  let cursor = new Date(now);
  cursor.setUTCSeconds(0, 0);
  cursor = new Date(cursor.getTime() + MINUTE_MS);

  for (let step = 0; step < CRON_SCAN_CAP && upcoming.length < count; step += 1) {
    const dayOfMonth = cursor.getUTCDate();
    const dayOfWeek = cursor.getUTCDay();
    const dayMatches =
      !parsed.dayOfMonth.any && !parsed.dayOfWeek.any
        ? parsed.dayOfMonth.set.has(dayOfMonth) || parsed.dayOfWeek.set.has(dayOfWeek)
        : matches(parsed.dayOfMonth, dayOfMonth) && matches(parsed.dayOfWeek, dayOfWeek);

    if (
      matches(parsed.minute, cursor.getUTCMinutes()) &&
      matches(parsed.hour, cursor.getUTCHours()) &&
      matches(parsed.month, cursor.getUTCMonth() + 1) &&
      dayMatches
    ) {
      upcoming.push(new Date(cursor.getTime()));
    }
    cursor = new Date(cursor.getTime() + MINUTE_MS);
  }
  return upcoming;
}
