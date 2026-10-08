/**
 * Detail-page derivations shared by both daemon entities: the Starts row of a
 * schedule (headline, faint line, next runs), the pause line, the delete
 * consequence, and the plain words the head and rail use. Pure derivation.
 */

import {
  automationDoesOf,
  automationStartOf,
  capitalize,
  formatOnceAt,
  zoneLabel,
} from "./automation-sentence";
import { isAutomationTrigger, type AutomationEntity } from "./automation-entity";
import type { AutomationDoes, AutomationStart, SentenceContext } from "./automation-sentence";
import { projectAutomationTarget } from "./automation-target";
import { cronNext, formatAbsoluteUtc, formatRelative, humanCron } from "./cron-engine";
import { humanizeFireWindow } from "./automation-formatters";
import type { AutomationJob, AutomationRun } from "../types";

export interface AutomationNextRun {
  index: number;
  relative: string;
  absolute: string;
  isFirst: boolean;
  oneTime: boolean;
}

export interface ScheduleStarts {
  headline: string;
  /** Faint line: days, zone and the raw expression, or the `every` / past `at` note. */
  sub: string;
  /** Raw cron expression shown as code inside `sub`; absent for `every` / `at`. */
  expression: string | null;
  /** Upcoming fire times; empty when Off, past, or counted from turn-on. */
  nextRuns: AutomationNextRun[];
}

const DETAIL_NEXT_RUNS = 3;

function mapNextRuns(dates: Date[], now: number, oneTime: boolean): AutomationNextRun[] {
  return dates.map((date, index) => ({
    index: index + 1,
    relative: formatRelative(date, now),
    absolute: formatAbsoluteUtc(date),
    isFirst: index === 0,
    oneTime,
  }));
}

/** `Monday to Friday` / `Saturday and Sunday` for the two day sets that read better spelled out. */
function dayRangePhrase(expr: string): string | null {
  const dow = expr.trim().split(/\s+/)[4];
  if (dow === "1-5" || dow === "1,2,3,4,5") return "Monday to Friday";
  if (dow === "0,6" || dow === "6,0") return "Saturday and Sunday";
  return null;
}

/**
 * The Starts row of a schedule. Next runs come from the cron engine (UTC) only
 * while the job is on and the zone is UTC; otherwise the daemon's own next run
 * stands alone, so the page never invents a time it cannot back.
 */
export function describeScheduleStarts(
  job: AutomationJob,
  ctx: SentenceContext = {},
  now: number = Date.now()
): ScheduleStarts {
  const schedule = job.schedule;
  const zone = zoneLabel(ctx);
  const daemonNext = job.scheduler?.next_run_at ?? job.next_run ?? null;
  const fallbackNext =
    job.enabled && daemonNext ? mapNextRuns([new Date(daemonNext)], now, false) : [];
  switch (schedule?.mode) {
    case "cron": {
      const expr = schedule.expr ?? "";
      const human = humanCron(expr);
      const days = dayRangePhrase(expr);
      const sub = [days, `times in ${zone}`].filter(Boolean).join(" · ");
      const computed = zone === "UTC" ? cronNext(expr, DETAIL_NEXT_RUNS, now) : null;
      return {
        headline: human ? capitalize(human) : "On a custom schedule",
        sub: capitalize(sub),
        expression: expr || null,
        nextRuns: job.enabled ? (computed ? mapNextRuns(computed, now, false) : fallbackNext) : [],
      };
    }
    case "every":
      return {
        headline: `Every ${humanizeFireWindow(schedule.interval ?? "")}`,
        sub: "Starts counting from when it was turned on",
        expression: null,
        nextRuns: [],
      };
    case "at": {
      const at = schedule.time ? new Date(schedule.time) : null;
      const past = !at || Number.isNaN(at.getTime()) || at.getTime() <= now;
      return {
        headline: schedule.time ? `Once, on ${formatOnceAt(schedule.time, ctx)}` : "Once",
        sub: past ? "Already ran" : capitalize(`times in ${zone}`),
        expression: null,
        nextRuns: !past && job.enabled && at ? mapNextRuns([at], now, true) : [],
      };
    }
    default:
      return {
        headline: "Manual",
        sub: "Runs only when started by hand",
        expression: null,
        nextRuns: [],
      };
  }
}

const START_WORDS = {
  schedule: "On a schedule",
  event: "On an event",
  webhook: "From a link",
} as const satisfies Record<AutomationStart, string>;

const DOES_WORDS = {
  agent: "Ask an agent",
  loop: "Start a Loop",
  task: "Create a task",
} as const satisfies Record<AutomationDoes, string>;

/** Subhead kind word: `On a schedule` · `On an event` · `From a link`. */
export function automationStartWord(start: AutomationStart): string {
  return START_WORDS[start];
}

/** Rail Details "Does" value. */
export function automationDoesWord(does: AutomationDoes): string {
  return DOES_WORDS[does];
}

/** Shown under the sentence while the automation is Off. */
export function automationPauseLine(start: AutomationStart): string {
  return start === "schedule"
    ? "Off. It won't run on its schedule until you turn it on."
    : "Off. Matching events won't start it until you turn it on.";
}

/** What stops happening once the automation is deleted; history always stays. */
export function automationDeleteConsequence(entity: AutomationEntity): string {
  if (automationStartOf(entity) !== "schedule") {
    return "Matching events will stop starting it. Past runs stay in the log.";
  }
  const target = projectAutomationTarget(entity);
  const stops =
    automationDoesOf(entity) === "task"
      ? "creating tasks"
      : target.kind === "loop"
        ? `starting the Loop ${target.loopName}`
        : `asking ${target.agentName}`;
  return `Its schedule will stop ${stops}. Past runs stay in the log.`;
}

/** Newest actual run start, excluding scheduled reservations that have not begun. */
export function automationLastRanAt(runs: readonly AutomationRun[]): string | null {
  let newest: string | null = null;
  for (const run of runs) {
    if (run.status === "scheduled" || !run.started_at) continue;
    if (newest === null || run.started_at > newest) newest = run.started_at;
  }
  return newest;
}

/** CLI read of this automation: `compozy automation jobs get morning-digest`. */
export function automationCliHint(entity: AutomationEntity): string {
  return `compozy automation ${isAutomationTrigger(entity) ? "triggers" : "jobs"} get ${entity.id}`;
}
