import type { AgentInstanceCheckIn, AgentInstancePayload } from "../types";

export type DayCell = "quiet" | "told" | "failed" | "next" | "later";

export interface PastCheckIn {
  at: string;
  cell: DayCell;
}

export interface RecentCheckIn {
  key: string;
  time: string;
  title: string;
  description?: string;
  told?: boolean;
  failed?: boolean;
}

export interface CheckInActivity {
  past: readonly PastCheckIn[];
  recent: readonly RecentCheckIn[];
}

/** Every wake reason and check gate, in plain words. */
const REASON: Record<string, string> = {
  quiet: "Nothing new, so it didn't bother you.",
  digest_empty: "Nothing new, so it didn't bother you.",
  digest_unchanged: "Nothing changed since last time.",
  reported: "It told you in its main chat.",
  wake_sent: "It told you in its main chat.",
  cancelled: "This check was stopped.",
  hook_cancelled: "A hook stopped this check.",
  policy_disabled: "Check-ins are off in its settings.",
  heartbeat_disabled: "Check-ins are off in its settings.",
  heartbeat_invalid: "Its check-in settings have a mistake.",
  heartbeat_no_policy: "It has no check-in settings yet.",
  heartbeat_rate_limited: "It checked in very recently, so it waited.",
  heartbeat_no_eligible_session: "It had no chat to check in from.",
  definition_not_stays_on: "This agent no longer stays on.",
  definition_missing: "This agent's definition is gone.",
  no_slot: "Too many checks were running, so this one waited.",
  duplicate_suppressed: "It already told you this recently.",
  triage_unsupported: "Its model can't do check-ins.",
  triage_output_invalid: "It gave an answer it couldn't use.",
  triage_timeout: "It ran out of time.",
  triage_interrupted: "It was interrupted partway through.",
  triage_failed: "It lost its connection partway through.",
  cooldown_active: "It checked in very recently, so it waited.",
  quiet_window: "It was outside its hours.",
  session_not_found: "Its main chat is gone.",
  session_unhealthy: "Its main chat is having trouble.",
  session_not_attachable: "It couldn't reach its main chat.",
  session_prompt_active: "It was busy in its main chat, so it skipped this one.",
  session_prompt_active_race: "It was busy in its main chat, so it skipped this one.",
  synthetic_prompt_failed: "It couldn't reach its main chat.",
  wake_coalesced: "It joined a check that was already running.",
};

export function checkInReason(reason: string | null | undefined) {
  return REASON[reason ?? ""] ?? "It skipped this one.";
}

function isTold(checkIn: AgentInstanceCheckIn) {
  return checkIn.result === "reported" || checkIn.result === "sent";
}

function cellOf(checkIn: AgentInstanceCheckIn): DayCell {
  if (checkIn.result === "failed") return "failed";
  return isTold(checkIn) ? "told" : "quiet";
}

function isToday(at: string, now: Date) {
  const date = new Date(at);
  return (
    date.getFullYear() === now.getFullYear() &&
    date.getMonth() === now.getMonth() &&
    date.getDate() === now.getDate()
  );
}

const CLOCK = new Intl.DateTimeFormat("en-GB", {
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

export function clockAt(at: string) {
  return CLOCK.format(new Date(at));
}

function single(checkIn: AgentInstanceCheckIn): RecentCheckIn {
  const base = { key: checkIn.at, time: clockAt(checkIn.at) };
  const sentence = checkInReason(checkIn.reason);
  if (checkIn.result === "failed") {
    return {
      ...base,
      title: "Couldn't finish",
      description: checkIn.detail || sentence,
      failed: true,
    };
  }
  if (isTold(checkIn)) {
    return { ...base, title: "Told you something", description: checkIn.detail, told: true };
  }
  return { ...base, title: "All quiet", description: checkIn.detail || sentence };
}

function quietRun(run: readonly AgentInstanceCheckIn[]): RecentCheckIn {
  const newest = run[0]!;
  const oldest = run[run.length - 1]!;
  return {
    key: newest.at,
    time: `${clockAt(oldest.at)} to ${clockAt(newest.at)}`,
    title: `${run.length} quiet check-ins`,
  };
}

/** Today, oldest first for the strip and newest first with quiet runs folded for the list. */
export function checkInActivity(
  checkIns: readonly AgentInstanceCheckIn[],
  now: Date
): CheckInActivity {
  const today = checkIns
    .filter(item => isToday(item.at, now))
    .sort((a, b) => b.at.localeCompare(a.at));
  const recent: RecentCheckIn[] = [];
  let run: AgentInstanceCheckIn[] = [];
  const flush = () => {
    if (run.length === 1) recent.push(single(run[0]!));
    if (run.length > 1) recent.push(quietRun(run));
    run = [];
  };
  for (const item of today) {
    if (cellOf(item) === "quiet") {
      run.push(item);
      continue;
    }
    flush();
    recent.push(single(item));
  }
  flush();
  return {
    past: [...today].reverse().map(item => ({ at: item.at, cell: cellOf(item) })),
    recent,
  };
}

/** How the chosen project stands, in the order the page has to say it. */
export type CheckInStanding = "running" | "paused" | "failed" | "slowed" | "off";

const SLOWED_RATIO = 1.5;

/** The real gap after a quiet streak, when the daemon has stretched the schedule. */
export function slowedMinutes(instance: AgentInstancePayload, intervalMinutes: number) {
  if (!instance.quiet_streak || !instance.last_wake_at || !instance.next_wake_at) return null;
  const gap =
    (new Date(instance.next_wake_at).getTime() - new Date(instance.last_wake_at).getTime()) /
    60_000;
  return gap >= intervalMinutes * SLOWED_RATIO ? Math.round(gap) : null;
}

export function checkInStanding(
  instance: AgentInstancePayload,
  intervalMinutes: number
): CheckInStanding {
  if (instance.state === "paused") {
    return instance.pause_reason === "fatigue" ? "failed" : "paused";
  }
  // An instance from before checks existed has no flag, and stays off until it's turned on.
  if (instance.checks_enabled !== true) return "off";
  return slowedMinutes(instance, intervalMinutes) === null ? "running" : "slowed";
}

export function gapLabel(minutes: number) {
  if (minutes < 60) return `${minutes} minutes`;
  const hours = Math.round(minutes / 60);
  return hours === 1 ? "hour" : `${hours} hours`;
}
