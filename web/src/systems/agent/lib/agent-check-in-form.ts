import type { AgentHeartbeatPayload } from "../types";
import type { DayCell, PastCheckIn } from "./agent-check-in-activity";
import { serializeAgentHeartbeatSource } from "./agent-authored-file-source";
import {
  markdownBlock,
  parseFrontmatter,
  splitFrontmatter,
  type FileBlock,
  type FrontmatterMap,
  type FrontmatterValue,
} from "./agent-frontmatter";

export const HOW_OFTEN = [
  { label: "5 min", minutes: 5 },
  { label: "15 min", minutes: 15 },
  { label: "30 min", minutes: 30 },
  { label: "1 hr", minutes: 60 },
  { label: "2 hr", minutes: 120 },
  { label: "4 hr", minutes: 240 },
] as const;

export const LOOK_AT_IDS = [
  "task",
  "session_health",
  "sessions_idle",
  "automations_failed",
  "monitors_fired",
] as const;

export const LUNCH: readonly [number, number] = [12, 13];
const ALL_DAY: readonly [number, number] = [0, 24];
const DEFAULT_STOP = 2;

/** HEARTBEAT.md as Check-ins shows it, with what the page must carry through a save. */
export interface CheckInSettings {
  stop: number;
  hours: readonly [number, number];
  lunch: boolean;
  looksAt: readonly string[];
  also: string;
  timezone: string;
  kept: {
    version: number;
    enabled: boolean;
    summary?: string;
    /** The file's own spelling, reused until the slider moves off `stop`. */
    interval: { text: string | null; stop: number };
  };
}

export interface CheckInDefaults {
  /** The stop that matches `agents.heartbeat.default_interval`. */
  defaultStop: number;
  timezone: string;
}

export function localTimezone() {
  return Intl.DateTimeFormat().resolvedOptions().timeZone;
}

export function blankCheckIns({ defaultStop, timezone }: CheckInDefaults): CheckInSettings {
  return {
    stop: defaultStop,
    hours: [9, 18],
    lunch: true,
    looksAt: ["task", "session_health", "sessions_idle"],
    also: "",
    timezone,
    kept: { version: 1, enabled: true, interval: { text: null, stop: -1 } },
  };
}

/** A half-hour step of the day as `09:30`. */
export function formatClock(hour: number) {
  const whole = Math.floor(hour);
  return `${String(whole).padStart(2, "0")}:${hour % 1 === 0 ? "00" : "30"}`;
}

function formatMinutes(hour: number) {
  const minutes = Math.round(hour * 60);
  const mm = String(minutes % 60).padStart(2, "0");
  return `${String(Math.floor(minutes / 60)).padStart(2, "0")}:${mm}`;
}

/** A Go duration like `30m` or `1h30m`, in minutes; `null` when it isn't one. */
export function durationMinutes(text: string | null | undefined) {
  const match = text?.trim().match(/^(?:(\d+)h)?(?:(\d+)m)?(?:0s)?$/);
  if (!match || (match[1] === undefined && match[2] === undefined)) return null;
  return Number(match[1] ?? 0) * 60 + Number(match[2] ?? 0);
}

/** The first stop the config lets the slider reach; an unknown limit starts at the first stop. */
export function floorStop(minInterval: string | null | undefined) {
  const minutes = durationMinutes(minInterval);
  if (minutes === null) return 0;
  const index = HOW_OFTEN.findIndex(stop => stop.minutes >= minutes);
  return index < 0 ? HOW_OFTEN.length - 1 : index;
}

function stopOf(minutes: number | null) {
  return HOW_OFTEN.findIndex(stop => stop.minutes === minutes);
}

function halfHour(text: FrontmatterValue | undefined) {
  if (typeof text !== "string") return null;
  const match = /^(\d{2}):(00|30)$/.exec(text.trim());
  if (!match) return null;
  const hour = Number(match[1]) + (match[2] === "30" ? 0.5 : 0);
  return hour < 24 ? hour : null;
}

interface Window {
  timezone: string;
  range: readonly [number, number];
}

function readWindow(value: FrontmatterValue): Window | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return null;
  const window = value as FrontmatterMap;
  const start = halfHour(window.start);
  const end = halfHour(window.end);
  if (start === null || end === null || typeof window.timezone !== "string") return null;
  // A window ending at midnight wraps on the server; any other wrap has no place on the slider.
  const close = end === 0 && start > 0 ? 24 : end;
  return close > start ? { timezone: window.timezone, range: [start, close] } : null;
}

/** One window at most, or `undefined` when the form can't draw what's there. */
function readWindows(value: FrontmatterValue | undefined) {
  if (value === undefined || value === null) return null;
  if (!Array.isArray(value)) return undefined;
  const items = value as readonly FrontmatterValue[];
  if (items.length === 0) return null;
  if (items.length > 1) return undefined;
  return readWindow(items[0]!) ?? undefined;
}

function hasOnlyKeys(map: FrontmatterMap, keys: readonly string[]) {
  const allowed = new Set(keys);
  return Object.keys(map).every(key => allowed.has(key));
}

function asMap(value: FrontmatterValue | undefined): FrontmatterMap | null {
  if (value === undefined || value === null) return {};
  if (typeof value !== "object" || Array.isArray(value)) return null;
  return value as FrontmatterMap;
}

export interface ParsedCheckIns {
  settings: CheckInSettings;
  block: FileBlock | null;
}

function readInclude(context: FrontmatterMap) {
  const include = context.include ?? [];
  if (!Array.isArray(include)) return null;
  const items = include as readonly FrontmatterValue[];
  const known: readonly string[] = LOOK_AT_IDS;
  return items.every(item => typeof item === "string" && known.includes(item))
    ? (items as readonly string[])
    : null;
}

function readSettings(
  data: FrontmatterMap,
  body: string,
  defaults: CheckInDefaults
): CheckInSettings | null {
  const preferences = asMap(data.preferences);
  const context = asMap(data.context);
  if (!preferences || !context) return null;
  if (!hasOnlyKeys(data, ["version", "enabled", "summary", "preferences", "context"])) return null;
  if (!hasOnlyKeys(preferences, ["min_interval", "active_hours", "quiet_windows"])) return null;
  if (!hasOnlyKeys(context, ["include"])) return null;
  const looksAt = readInclude(context);
  const active = readWindows(preferences.active_hours);
  const quiet = readWindows(preferences.quiet_windows);
  if (!looksAt || active === undefined || quiet === undefined) return null;
  if (quiet && (quiet.range[0] !== LUNCH[0] || quiet.range[1] !== LUNCH[1])) return null;
  if (active && quiet && active.timezone !== quiet.timezone) return null;
  const text = typeof preferences.min_interval === "string" ? preferences.min_interval : null;
  const stop = text === null ? defaults.defaultStop : stopOf(durationMinutes(text));
  if (stop < 0) return null;
  return {
    stop,
    hours: active?.range ?? ALL_DAY,
    lunch: quiet !== null,
    looksAt,
    also: body,
    timezone: active?.timezone ?? quiet?.timezone ?? defaults.timezone,
    kept: {
      version: typeof data.version === "number" ? data.version : 1,
      enabled: data.enabled !== false,
      ...(typeof data.summary === "string" ? { summary: data.summary } : {}),
      interval: { text, stop },
    },
  };
}

export function parseCheckInsFile(source: string, defaults: CheckInDefaults): ParsedCheckIns {
  const blank = blankCheckIns(defaults);
  const file = splitFrontmatter(source);
  if (!file) return { settings: blank, block: "mistake" };
  const data = file.metadata === null ? {} : parseFrontmatter(file.metadata);
  if (!data) return { settings: blank, block: "settings" };
  const body = file.body.trimEnd();
  const block = markdownBlock(body);
  if (block) return { settings: blank, block };
  const settings = readSettings(data, body, defaults);
  return settings ? { settings, block: null } : { settings: blank, block: "settings" };
}

function clock(hour: number) {
  return hour === 24 ? "00:00" : formatClock(hour);
}

export function composeCheckInsFile(settings: CheckInSettings) {
  const { kept, hours, timezone } = settings;
  const allDay = hours[0] === ALL_DAY[0] && hours[1] === ALL_DAY[1];
  const window = (range: readonly [number, number]) => ({
    timezone,
    start: clock(range[0]),
    end: clock(range[1]),
  });
  return serializeAgentHeartbeatSource({
    guidance_markdown: settings.also,
    frontmatter: {
      version: kept.version,
      enabled: kept.enabled,
      summary: kept.summary,
      preferences: {
        min_interval:
          settings.stop === kept.interval.stop
            ? (kept.interval.text ?? undefined)
            : `${HOW_OFTEN[settings.stop]!.minutes}m`,
        active_hours: allDay ? [] : [window(hours)],
        quiet_windows: settings.lunch ? [window(LUNCH)] : [],
      },
      context: { include: [...settings.looksAt] },
    },
  });
}

export function checkInsFromPayload(payload: AgentHeartbeatPayload, timezone = localTimezone()) {
  const digest = payload.digest ?? "";
  const subset = payload.config_provenance?.subset;
  const stop = stopOf(durationMinutes(subset?.default_interval));
  const defaults = { defaultStop: stop < 0 ? DEFAULT_STOP : stop, timezone };
  if (!payload.present) {
    return { digest, source: "", form: blankCheckIns(defaults), block: null, missing: true };
  }
  const source = serializeAgentHeartbeatSource(payload);
  if (!payload.valid) {
    const form = blankCheckIns(defaults);
    return { digest, source, form, block: "mistake" as const, missing: false };
  }
  const { settings, block } = parseCheckInsFile(source, defaults);
  return { digest, source, form: settings, block, missing: false };
}

export function countCheckInChanges(a: CheckInSettings, b: CheckInSettings) {
  return [
    a.stop !== b.stop,
    a.hours.join() !== b.hours.join(),
    a.lunch !== b.lunch,
    a.looksAt.join() !== b.looksAt.join(),
    a.also !== b.also,
  ].filter(Boolean).length;
}

export function everyLabel(stop: number) {
  return `Every ${HOW_OFTEN[stop]!.label}`;
}

export function scheduleSummary(settings: CheckInSettings, on: boolean) {
  if (!on) return "Off";
  const [start, end] = settings.hours;
  return `${everyLabel(settings.stop)}, ${formatClock(start)} to ${formatClock(end)}`;
}

function resting(hour: number, settings: CheckInSettings) {
  return settings.lunch && hour >= LUNCH[0] && hour < LUNCH[1];
}

function upcoming(settings: CheckInSettings, now: number) {
  const step = HOW_OFTEN[settings.stop]!.minutes / 60;
  const [start, end] = settings.hours;
  const slots: number[] = [];
  for (let hour = start; hour < end; hour += step) {
    if (hour > now && !resting(hour, settings)) slots.push(hour);
  }
  return slots;
}

export interface NextCheckIn {
  day: "Today" | "Tomorrow";
  time: string;
  /** Plain distance from now, for today only. */
  distance?: string;
}

function distance(minutes: number) {
  if (minutes < 60) return `in ${minutes} min`;
  const whole = Math.floor(minutes / 60);
  const rest = minutes % 60;
  return rest === 0 ? `in ${whole} hr` : `in ${whole} hr ${rest} min`;
}

/** What the schedule on screen would do. `fresh`: just turned on, so one gap from now. */
export function previewNextCheckIn(
  settings: CheckInSettings,
  now: number,
  fresh = false
): NextCheckIn {
  const step = HOW_OFTEN[settings.stop]!.minutes / 60;
  const wanted = fresh ? now + step : undefined;
  const hour =
    wanted !== undefined && wanted < settings.hours[1] ? wanted : upcoming(settings, now)[0];
  if (hour === undefined) return { day: "Tomorrow", time: formatMinutes(settings.hours[0]) };
  return {
    day: "Today",
    time: formatMinutes(hour),
    distance: distance(Math.round((hour - now) * 60)),
  };
}

function dayKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
}

/** The server's `next_wake_at`, read the way the preview reads. */
export function scheduledNextCheckIn(at: string, current: Date): NextCheckIn | null {
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return null;
  const hour = date.getHours() + date.getMinutes() / 60;
  const tomorrow = new Date(current);
  tomorrow.setDate(current.getDate() + 1);
  const today = dayKey(date) === dayKey(current);
  if (!today && dayKey(date) !== dayKey(tomorrow)) return null;
  return {
    day: today ? "Today" : "Tomorrow",
    time: formatMinutes(hour),
    ...(today
      ? {
          distance: distance(
            Math.max(1, Math.round((date.getTime() - current.getTime()) / 60_000))
          ),
        }
      : {}),
  };
}

/** Hours since midnight, on the file's wall clock. */
export function hourInZone(date: Date, timezone: string) {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: timezone,
    hour: "numeric",
    minute: "numeric",
    hourCycle: "h23",
  }).formatToParts(date);
  const read = (type: string) => Number(parts.find(part => part.type === type)?.value ?? 0);
  return read("hour") + read("minute") / 60;
}

/** Today's strip: what happened, then what's still ahead on the schedule on screen. */
export function dayCells(
  settings: CheckInSettings,
  past: readonly PastCheckIn[],
  now: number,
  running: boolean
) {
  const ahead = running ? upcoming(settings, now) : [];
  const cells: DayCell[] = [
    ...past.map(item => item.cell),
    ...ahead.map((_, index): DayCell => (index === 0 ? "next" : "later")),
  ];
  return { cells, done: past.length, total: cells.length };
}
