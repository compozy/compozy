/** What the Check-ins page derives: the summary, the next check-in and today's strip. */
import { formatClock } from "../stays-on-agents-shared/format-clock";
import type { DayCell } from "../stays-on-agents-shared/rich-controls";

export const HOW_OFTEN = [
  { label: "5 min", minutes: 5 },
  { label: "15 min", minutes: 15 },
  { label: "30 min", minutes: 30 },
  { label: "1 hr", minutes: 60 },
  { label: "2 hr", minutes: 120 },
  { label: "4 hr", minutes: 240 },
] as const;

/** The shortest gap the operator's config allows. */
export const FLOOR = 1;
/** The fixtures' clock: 14:12. */
export const NOW = 14 + 12 / 60;
export const LUNCH: readonly [number, number] = [12, 13];

export interface CheckInSettings {
  on: boolean;
  stop: number;
  hours: readonly [number, number];
  lunch: boolean;
  looksAt: readonly string[];
  also: string;
}

export interface PastCheckIn {
  hour: number;
  cell: DayCell;
}

export interface PlaceActivity {
  past: readonly PastCheckIn[];
  recent: readonly {
    time: string;
    title: string;
    description?: string;
    told?: boolean;
    failed?: boolean;
  }[];
}

export type Standing = "running" | "paused" | "failed" | "slowed" | "nowhere";
export type TryPhase = "idle" | "running" | "result" | "sent";

export function everyLabel(stop: number) {
  return `Every ${HOW_OFTEN[stop]!.label}`;
}

export function scheduleSummary(settings: CheckInSettings) {
  if (!settings.on) return "Off";
  const [start, end] = settings.hours;
  return `${everyLabel(settings.stop)}, ${formatClock(start)} to ${formatClock(end)}`;
}

function resting(hour: number, settings: CheckInSettings) {
  return settings.lunch && hour >= LUNCH[0] && hour < LUNCH[1];
}

function upcoming(settings: CheckInSettings) {
  const step = HOW_OFTEN[settings.stop]!.minutes / 60;
  const [start, end] = settings.hours;
  const slots: number[] = [];
  for (let hour = start; hour < end; hour += step) {
    if (hour > NOW && !resting(hour, settings)) slots.push(hour);
  }
  return slots;
}

export interface NextCheckIn {
  day: "Today" | "Tomorrow";
  time: string;
  /** Plain distance from now, for today only. */
  distance?: string;
}

function distance(hour: number) {
  const minutes = Math.round((hour - NOW) * 60);
  if (minutes < 60) return `in ${minutes} min`;
  const whole = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `in ${whole} hr` : `in ${whole} hr ${rest} min`;
}

/** `fresh`: just turned on, so the first one comes one gap from now. */
export function nextCheckIn(settings: CheckInSettings, fresh = false): NextCheckIn | null {
  if (!settings.on) return null;
  const step = HOW_OFTEN[settings.stop]!.minutes / 60;
  const [start, end] = settings.hours;
  const wanted = fresh ? NOW + step : undefined;
  const hour = wanted !== undefined && wanted < end ? wanted : upcoming(settings)[0];
  if (hour === undefined) return { day: "Tomorrow", time: formatMinutes(start) };
  return { day: "Today", time: formatMinutes(hour), distance: distance(hour) };
}

function formatMinutes(hour: number) {
  const minutes = Math.round(hour * 60);
  const mm = String(minutes % 60).padStart(2, "0");
  return `${String(Math.floor(minutes / 60)).padStart(2, "0")}:${mm}`;
}

export function dayCells(settings: CheckInSettings, past: readonly PastCheckIn[]) {
  const ahead = settings.on ? upcoming(settings) : [];
  const cells: DayCell[] = [
    ...past.map(item => item.cell),
    ...ahead.map((_, index): DayCell => (index === 0 ? "next" : "later")),
  ];
  return { cells, done: past.length, total: cells.length };
}

export function countChanges(a: CheckInSettings, b: CheckInSettings) {
  return [
    a.on !== b.on,
    a.stop !== b.stop,
    a.hours.join() !== b.hours.join(),
    a.lunch !== b.lunch,
    a.looksAt.join() !== b.looksAt.join(),
    a.also !== b.also,
  ].filter(Boolean).length;
}

export function composeFile(settings: CheckInSettings) {
  const interval = HOW_OFTEN[settings.stop]!;
  return [
    "---",
    "version: 1",
    `enabled: ${settings.on}`,
    "preferences:",
    `  min_interval: ${interval.minutes}m`,
    "  active_hours:",
    "    - timezone: America/Sao_Paulo",
    `      start: "${formatClock(settings.hours[0])}"`,
    `      end: "${formatClock(settings.hours[1])}"`,
    ...(settings.lunch
      ? [
          "  quiet_windows:",
          "    - timezone: America/Sao_Paulo",
          `      start: "${formatClock(LUNCH[0])}"`,
          `      end: "${formatClock(LUNCH[1])}"`,
        ]
      : []),
    "context:",
    `  include: [${settings.looksAt.join(", ")}]`,
    "---",
    settings.also,
  ].join("\n");
}
