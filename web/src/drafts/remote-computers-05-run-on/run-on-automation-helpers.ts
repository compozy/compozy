import { Bot, Clock3, Radio, Repeat2, SquareCheck, Webhook } from "@compozy/ui/icons";

import type { AutomationChoice } from "@/systems/automation/components/automation-form/automation-choice-cards";
import {
  compileCron,
  cronNext,
  decodeCron,
  formatRelative,
  localInputToDate,
  parseCron,
  parseDuration,
} from "@/systems/automation/lib/cron-engine";
import { humanCron } from "@/systems/automation/lib/cron-engine-presentation";
import type { AutomationSchedule } from "@/systems/automation/types";

export type DraftAutomationStart = "schedule" | "event" | "webhook";
export type DraftAutomationDoes = "agent" | "always-on" | "loop" | "task";

export const SCHEDULE_QUICK_PICKS = [
  { label: "Weekdays 9am", expr: "0 9 * * 1-5" },
  { label: "Every day 9am", expr: "0 9 * * *" },
  { label: "Mondays 8am", expr: "0 8 * * 1" },
  { label: "Every hour", expr: "0 * * * *" },
  { label: "Midnight", expr: "0 0 * * *" },
] as const;

export const SCHEDULE_EVERY_PICKS = ["5m", "15m", "30m", "1h", "4h", "24h"] as const;
export const CRON_PARTS_HINT = "min · hour · day · month · weekday";
export const ALL_DAYS = [0, 1, 2, 3, 4, 5, 6] as const;

export const AUTOMATION_START_CHOICES: readonly AutomationChoice<DraftAutomationStart>[] = [
  {
    value: "schedule",
    icon: Clock3,
    title: "On a schedule",
    description: "Every morning, every hour, or once at a set time.",
  },
  {
    value: "event",
    icon: Radio,
    title: "When something happens",
    description: "A session starts or stops, a hook finishes, an extension sends an event.",
  },
  {
    value: "webhook",
    icon: Webhook,
    title: "When another app calls a link",
    description: "GitHub, CI or any service sends a signed request.",
  },
];

export const AUTOMATION_DOES_CHOICES: readonly AutomationChoice<DraftAutomationDoes>[] = [
  {
    value: "agent",
    icon: Bot,
    title: "Ask an agent",
    description: "Send a message to an agent in a new session.",
  },
  {
    value: "always-on",
    icon: Radio,
    title: "Tell an always-on agent",
    description: "Its next check reads the event and proposes what to do.",
    lockedReason: "Only events and links can reach an always-on agent",
  },
  {
    value: "loop",
    icon: Repeat2,
    title: "Start a Loop",
    description: "Run one of your Loops with set inputs.",
  },
  {
    value: "task",
    icon: SquareCheck,
    title: "Create a task",
    description: "Add a task for an agent pool or a person.",
  },
];

export function scheduleDayTime(expr: string) {
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

export function compileDayTime(days: readonly number[], hour: number, minute: number): string {
  const sorted = [...new Set(days)].sort((a, b) => a - b);
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

export function describeSchedulePhrase(schedule: AutomationSchedule, timeZone: string): string {
  if (schedule.mode === "cron") {
    const human = schedule.expr ? humanCron(schedule.expr) : null;
    if (!human) return "On a custom schedule";
    const cap = human.charAt(0).toUpperCase() + human.slice(1);
    return /\d{2}:\d{2}|midnight/.test(human) ? `${cap} ${timeZone}` : cap;
  }
  if (schedule.mode === "every") {
    return schedule.interval ? `Every ${schedule.interval}` : "Every interval";
  }
  return schedule.time ? `Once at ${schedule.time} ${timeZone}` : "Once at a set time";
}

export function buildScheduleReadout(
  schedule: AutomationSchedule,
  now: number,
  daysCleared: boolean,
  timeZone: string
): { valid: boolean; text: string } {
  if (schedule.mode === "cron") {
    if (daysCleared) return { valid: false, text: "Pick at least one day." };
    const expr = schedule.expr ?? "";
    const parts = expr.trim().split(/\s+/).filter(Boolean);
    if (parts.length !== 5 || !parseCron(expr)) {
      return { valid: false, text: `Needs 5 valid parts: ${CRON_PARTS_HINT}.` };
    }
    const phrase = describeSchedulePhrase(schedule, timeZone);
    const next = cronNext(expr, 1, now)?.[0];
    return {
      valid: true,
      text: [phrase, ...(next ? [`next ${formatRelative(next, now)}`] : []), expr.trim()].join(
        " · "
      ),
    };
  }
  if (schedule.mode === "every") {
    const interval = schedule.interval?.trim() ?? "";
    return parseDuration(interval)
      ? { valid: true, text: `Runs every ${interval}, starting right after you save.` }
      : { valid: false, text: "Use a duration like 30m, 1h or 2h30m." };
  }
  const date = localInputToDate(schedule.time ?? "");
  if (!date) return { valid: false, text: "Pick a date and time." };
  return date.getTime() <= now
    ? { valid: false, text: "That time is in the past. It would never run." }
    : { valid: true, text: `Runs once (${schedule.time} ${timeZone}), then stops.` };
}
