import type { HomeOverview } from "../types";

export type ActivityFilter = "all" | "unattended";
export type ActivityResult = "allowed" | "declined" | "expired";
export type ActivityStarter =
  | "human"
  | "automation"
  | "daemon"
  | "extension"
  | "agent_session"
  | "app"
  | "unknown";

export interface PermissionRollupRow {
  day: string;
  agent_name: string;
  decision: string;
  count: number;
}

export interface PermissionEntry {
  id: string;
  session_id: string;
  agent_name: string;
  action: string;
  resource: string;
  decision: string;
  initiator_kind: string;
  initiator_id: string;
  timestamp: string;
}

export interface ActivityRow {
  id: string;
  day: string;
  when: string;
  action: string;
  detail: string;
  agentName: string;
  starter: ActivityStarter;
  source?: string;
  result: ActivityResult;
}

export interface ActivityDay extends Record<string, string | number> {
  date: string;
  allowed: number;
  declined: number;
  expired: number;
}

export interface ActivityModel {
  total: number;
  allowed: number;
  declined: number;
  expired: number;
  sentence: string;
  days: readonly ActivityDay[];
  starters: readonly { key: ActivityStarter; label: string; value: number }[];
  agents: readonly { name: string; value: number }[];
  pulse: HomeOverview["pulse"];
  rows: readonly ActivityRow[];
}

export const ACTIVITY_WINDOW_DAYS = 14;

/** The route's cap; the rollups stay the exact totals past it. */
export const ACTIVITY_ENTRIES_LIMIT = 1000;

export const STARTER_LABEL: Record<ActivityStarter, string> = {
  human: "You",
  automation: "Automation",
  daemon: "Check-in",
  extension: "Extension",
  agent_session: "Another agent",
  app: "App",
  unknown: "Unknown",
};

const STARTER_GROUP: Record<ActivityStarter, string> = {
  ...STARTER_LABEL,
  automation: "Automations",
  daemon: "Check-ins",
  extension: "Extensions",
  agent_session: "Other agents",
  app: "Apps",
};

const STARTERS = new Set<string>(Object.keys(STARTER_LABEL));

/** Rows from before initiators were recorded stay unknown, never guessed (US-024.EC-1). */
export function activityStarter(kind: string): ActivityStarter {
  return STARTERS.has(kind) ? (kind as ActivityStarter) : "unknown";
}

export function activityResult(decision: string): ActivityResult {
  const value = decision.toLowerCase();
  if (value.startsWith("allow") || value.startsWith("approve")) return "allowed";
  if (value.includes("expire") || value.includes("timeout") || value.includes("cancel")) {
    return "expired";
  }
  return "declined";
}

const ACTION_COPY: Record<string, { done: string; wanted: string }> = {
  execute: { done: "Ran a command", wanted: "Wanted to run a command" },
  edit: { done: "Edited a file", wanted: "Wanted to edit a file" },
  write: { done: "Wrote a file", wanted: "Wanted to write a file" },
  read: { done: "Read a file", wanted: "Wanted to read a file" },
  fetch: { done: "Looked something up", wanted: "Wanted to look something up" },
};

export function activityActionLabel(action: string, result: ActivityResult) {
  const copy = ACTION_COPY[action.toLowerCase()];
  if (copy) return result === "allowed" ? copy.done : copy.wanted;
  return action || "Used a tool";
}

function dayKey(date: Date) {
  return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
}

function dayLabel(at: Date, now: Date) {
  if (dayKey(at) === dayKey(now)) return "Today";
  const yesterday = new Date(now);
  yesterday.setDate(now.getDate() - 1);
  if (dayKey(at) === dayKey(yesterday)) return "Yesterday";
  return at.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function clock(at: Date) {
  return `${String(at.getHours()).padStart(2, "0")}:${String(at.getMinutes()).padStart(2, "0")}`;
}

export function activityRow(entry: PermissionEntry, now: Date): ActivityRow {
  const at = new Date(entry.timestamp);
  const result = activityResult(entry.decision);
  const starter = activityStarter(entry.initiator_kind);
  const named = starter === "automation" || starter === "extension";
  return {
    id: entry.id,
    day: dayLabel(at, now),
    when: clock(at),
    action: activityActionLabel(entry.action, result),
    detail: entry.resource,
    agentName: entry.agent_name,
    starter,
    source: named && entry.initiator_id ? entry.initiator_id : undefined,
    result,
  };
}

function dailySeries(rollups: readonly PermissionRollupRow[], until: Date) {
  const days: (ActivityDay & { key: string })[] = [];
  for (let back = ACTIVITY_WINDOW_DAYS - 1; back >= 0; back -= 1) {
    const day = new Date(until);
    day.setUTCDate(until.getUTCDate() - back);
    days.push({
      key: day.toISOString().slice(0, 10),
      date: day.toLocaleDateString("en-US", { month: "short", day: "numeric", timeZone: "UTC" }),
      allowed: 0,
      declined: 0,
      expired: 0,
    });
  }
  const daysByKey = new Map(days.map(day => [day.key, day]));
  for (const row of rollups) {
    const day = daysByKey.get(row.day);
    if (day) day[activityResult(row.decision)] += row.count;
  }
  return days;
}

function pulseOf(entries: readonly PermissionEntry[]): HomeOverview["pulse"] {
  const cells = new Map<string, { weekday: number; hour: number; events: number }>();
  for (const entry of entries) {
    const at = new Date(entry.timestamp);
    const key = `${at.getDay()}:${at.getHours()}`;
    const cell = cells.get(key) ?? { weekday: at.getDay(), hour: at.getHours(), events: 0 };
    cell.events += 1;
    cells.set(key, cell);
  }
  const buckets = [...cells.values()];
  const busiest = buckets.reduce<(typeof buckets)[number] | null>(
    (best, cell) => (best && best.events >= cell.events ? best : cell),
    null
  );
  return { window_days: ACTIVITY_WINDOW_DAYS, buckets, busiest: busiest ?? undefined };
}

function count<T>(rows: readonly T[], key: (row: T) => string) {
  const totals = new Map<string, number>();
  for (const row of rows) totals.set(key(row), (totals.get(key(row)) ?? 0) + 1);
  return totals;
}

export function deriveActivityModel(
  filter: ActivityFilter,
  rollups: readonly PermissionRollupRow[],
  entries: readonly PermissionEntry[],
  until: Date,
  now: Date
): ActivityModel {
  const days = dailySeries(rollups, until);
  const allowed = days.reduce((sum, day) => sum + day.allowed, 0);
  const declined = days.reduce((sum, day) => sum + day.declined, 0);
  const expired = days.reduce((sum, day) => sum + day.expired, 0);
  const total = allowed + declined + expired;
  const shown = total.toLocaleString("en-US");
  const perAgent = new Map<string, number>();
  for (const row of rollups) {
    perAgent.set(row.agent_name, (perAgent.get(row.agent_name) ?? 0) + row.count);
  }
  return {
    total,
    allowed,
    declined,
    expired,
    sentence:
      filter === "unattended"
        ? `${shown} actions ran with nobody in the conversation.`
        : `${shown} actions in the last ${ACTIVITY_WINDOW_DAYS} days.`,
    days,
    starters: [...count(entries, entry => activityStarter(entry.initiator_kind))]
      .map(([key, value]) => ({
        key: key as ActivityStarter,
        label: STARTER_GROUP[key as ActivityStarter],
        value,
      }))
      .sort((a, b) => b.value - a.value),
    agents: [...perAgent]
      .map(([name, value]) => ({ name, value }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 4),
    pulse: pulseOf(entries),
    rows: entries.map(entry => activityRow(entry, now)),
  };
}
