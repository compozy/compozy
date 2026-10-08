/**
 * Schedule builder model for the editor: quick picks, days plus a time, and the
 * readout under the builder. Cron stays one level deeper ("Edit expression");
 * the builder compiles days and a time to the expression the daemon stores.
 */

import type { CreateAutomationJobRequest } from "../types";
import { humanizeFireWindow } from "./automation-formatters";
import { describeSchedule } from "./automation-sentence";
import {
  compileCron,
  cronNext,
  decodeCron,
  formatAbsoluteUtc,
  formatClock,
  formatRelative,
  localInputToDate,
  parseCron,
  parseDuration,
} from "./cron-engine";

type JobSchedule = CreateAutomationJobRequest["schedule"];

export interface ScheduleQuickPick {
  label: string;
  expr: string;
}

export const SCHEDULE_QUICK_PICKS: readonly ScheduleQuickPick[] = [
  { label: "Weekdays 9am", expr: "0 9 * * 1-5" },
  { label: "Every day 9am", expr: "0 9 * * *" },
  { label: "Mondays 8am", expr: "0 8 * * 1" },
  { label: "Every hour", expr: "0 * * * *" },
  { label: "Midnight", expr: "0 0 * * *" },
];

export const SCHEDULE_EVERY_PICKS = ["5m", "15m", "30m", "1h", "4h", "24h"] as const;

export const CRON_PARTS_HINT = "min · hour · day · month · weekday";

const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6] as const;
const DEFAULT_TIME = { hour: 9, minute: 0 };

/** Days (0 = Sunday) and a clock the simple builder can show, or `null` for other shapes. */
export interface ScheduleDayTime {
  days: number[];
  hour: number;
  minute: number;
}

export function scheduleDayTime(expr: string): ScheduleDayTime | null {
  const model = decodeCron(expr);
  if (!model || model.hour === undefined || model.minute === undefined) return null;
  if (model.frequency === "daily") {
    return { days: [...ALL_DAYS], hour: model.hour, minute: model.minute };
  }
  if (model.frequency === "weekly" && model.weekdays) {
    return { days: [...model.weekdays], hour: model.hour, minute: model.minute };
  }
  return null;
}

function compileDayTime(days: readonly number[], hour: number, minute: number): string {
  const sorted = [...new Set(days)].sort((left, right) => left - right);
  return (
    compileCron({
      frequency: sorted.length === 7 ? "daily" : "weekly",
      everyMinutes: 15,
      hourlyMinute: 0,
      hour,
      minute,
      weekdays: sorted,
      monthDay: 1,
    }) ?? ""
  );
}

/**
 * Toggles one day. Clearing the last day keeps the stored expression and
 * reports `cleared`, so the sentence can show "on some days" as missing.
 */
export function toggleScheduleDay(
  expr: string,
  day: number,
  cleared: boolean
): { expr: string; cleared: boolean } {
  const current = scheduleDayTime(expr);
  const time = current ?? { ...DEFAULT_TIME, days: [] };
  const days = cleared ? [] : (current?.days ?? []);
  const next = days.includes(day) ? days.filter(value => value !== day) : [...days, day];
  if (next.length === 0) return { expr, cleared: true };
  return { expr: compileDayTime(next, time.hour, time.minute), cleared: false };
}

/** Sets the clock while keeping the chosen days (every day when the shape had none). */
export function setScheduleTime(expr: string, value: string): string | null {
  const match = /^(\d{1,2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hour = Number(match[1]);
  const minute = Number(match[2]);
  if (hour > 23 || minute > 59) return null;
  const days = scheduleDayTime(expr)?.days ?? [...ALL_DAYS];
  return compileDayTime(days, hour, minute);
}

/** `HH:MM` for the builder's time input. */
export function scheduleClock(expr: string): string {
  const current = scheduleDayTime(expr) ?? { ...DEFAULT_TIME, days: [] };
  return formatClock(current.hour, current.minute);
}

export interface ScheduleReadout {
  valid: boolean;
  text: string;
}

function cronReadout(expr: string, now: number): ScheduleReadout {
  const parts = expr.trim().split(/\s+/).filter(Boolean);
  if (parts.length !== 5) {
    return { valid: false, text: `Needs 5 parts: ${CRON_PARTS_HINT}.` };
  }
  if (!parseCron(expr)) {
    return { valid: false, text: `One of the parts is out of range: ${CRON_PARTS_HINT}.` };
  }
  const phrase = describeSchedule({ mode: "cron", expr });
  const next = cronNext(expr, 1, now)?.[0];
  return {
    valid: true,
    text: [phrase, ...(next ? [`next ${formatRelative(next, now)}`] : []), expr.trim()].join(" · "),
  };
}

/** Plain-language readout under the builder; `valid` drives the glyph and the readiness. */
export function scheduleReadout(
  schedule: JobSchedule,
  now: number,
  { daysCleared = false }: { daysCleared?: boolean } = {}
): ScheduleReadout {
  if (schedule.mode === "cron") {
    if (daysCleared) return { valid: false, text: "Pick at least one day." };
    return cronReadout(schedule.expr ?? "", now);
  }
  if (schedule.mode === "every") {
    const interval = schedule.interval?.trim() ?? "";
    return parseDuration(interval)
      ? {
          valid: true,
          text: `Runs every ${humanizeFireWindow(interval)}, starting right after you save.`,
        }
      : { valid: false, text: "Use a duration like 30m, 1h or 2h30m." };
  }
  const date = localInputToDate(schedule.time ?? "");
  if (!date) return { valid: false, text: "Pick a date and time." };
  if (date.getTime() <= now) {
    return { valid: false, text: "That time is in the past. It would never run." };
  }
  return {
    valid: true,
    text: `Runs once, ${formatRelative(date, now)} (${formatAbsoluteUtc(date)} UTC), then stops.`,
  };
}
