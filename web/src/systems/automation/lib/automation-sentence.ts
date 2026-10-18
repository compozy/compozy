/**
 * One sentence grammar for every automation surface (row, card, detail head,
 * editor sentence bar, palette subtitle, preview summary).
 *
 * Schedules: `<schedule phrase>, <does phrase>.`
 * Events:    `When <event phrase> in <project>[ with <conditions>], <does phrase>.`
 * Links:     `When another app calls the <slug> link[ with <conditions>], <does phrase>.`
 *
 * Pure derivation over a persisted job, a persisted trigger, or an editor draft.
 * Missing draft parts render as `missing` placeholder segments.
 */

import { humanCron } from "./cron-engine-presentation";
import { parseEventSelection } from "./trigger-event-id";
import { LOOP_TARGET_KIND } from "./automation-drafts";
import { isAutomationTrigger } from "./automation-entity";
import { humanizeFilterKey } from "./trigger-filter";
import { humanizeFireWindow } from "./automation-formatters";
import type { AutomationJob, AutomationSchedule, AutomationTrigger } from "../types";

export type AutomationStart = "schedule" | "event" | "webhook";
export type AutomationDoes = "agent" | "loop" | "task";

export interface AutomationSentenceSegment {
  text: string;
  /** Varying part (time, agent, Loop, task); rendered one step stronger than the glue. */
  emphasis: boolean;
  /** Draft part still to fill; rendered dashed and makes the draft "Needs a fix". */
  missing?: boolean;
}

export type AutomationSentence = readonly AutomationSentenceSegment[];

/** Minimal editor-side shape `describeAutomation` reads; the editor produces it. */
export interface AutomationDraft {
  start: AutomationStart | null;
  schedule?: {
    mode: AutomationSchedule["mode"];
    expr?: string;
    interval?: string;
    time?: string;
    /** Builder day selection (0 = Sunday); an empty selection reads "on some days". */
    days?: readonly number[];
  };
  /** Runtime event id for event starts (`session.stopped`, `hook.<name>.completed`, …). */
  event?: string;
  /** Exact-match conditions (`data.stop_reason` → `error`). */
  filter?: Readonly<Record<string, string>>;
  webhook?: { slug?: string; webhookId?: string };
  target: {
    kind: AutomationDoes | null;
    agentName?: string;
    prompt?: string;
    loopName?: string;
    loopInputs?: Readonly<Record<string, unknown>>;
    taskTitle?: string;
    taskOwner?: string;
  };
  workspaceId?: string;
}

export interface SentenceContext {
  /** The global `automation.timezone`; defaults to UTC. */
  timeZone?: string;
  /** Resolves a workspace id to its project name; falls back to the id. */
  workspaceName?: (workspaceId: string) => string | null | undefined;
}

const PROMPT_CLAUSE_MAX = 60;

function plain(text: string): AutomationSentenceSegment {
  return { text, emphasis: false };
}

function strong(text: string): AutomationSentenceSegment {
  return { text, emphasis: true };
}

function missing(text: string): AutomationSentenceSegment {
  return { text, emphasis: true, missing: true };
}

export function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** The automation time zone label; the daemon default is UTC. */
export function zoneLabel(ctx: SentenceContext): string {
  return ctx.timeZone?.trim() || "UTC";
}

const ONCE_AT_FORMATTERS = new Map<string, Intl.DateTimeFormat>();

function onceAtFormatter(timeZone: string): Intl.DateTimeFormat {
  let formatter = ONCE_AT_FORMATTERS.get(timeZone);
  if (!formatter) {
    formatter = new Intl.DateTimeFormat("en-US", {
      weekday: "short",
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
      hourCycle: "h23",
      timeZone,
    });
    ONCE_AT_FORMATTERS.set(timeZone, formatter);
  }
  return formatter;
}

function zonedParts(date: Date, timeZone: string): Record<string, string> | null {
  let parts: Intl.DateTimeFormatPart[];
  try {
    parts = onceAtFormatter(timeZone).formatToParts(date);
  } catch {
    return null;
  }
  return Object.fromEntries(parts.map(part => [part.type, part.value]));
}

/** `Thu Oct 8, 09:00` in the automation time zone (zone label not included). */
export function formatAbsoluteInZone(date: Date, ctx: SentenceContext = {}): string {
  const part = zonedParts(date, zoneLabel(ctx));
  if (!part) return date.toISOString();
  return `${part.weekday} ${part.month} ${part.day}, ${part.hour}:${part.minute}`;
}

/** `Thu Oct 8 at 09:00 UTC` in the automation time zone. */
export function formatOnceAt(iso: string, ctx: SentenceContext = {}): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const timeZone = zoneLabel(ctx);
  const part = zonedParts(date, timeZone);
  if (!part) return iso;
  return `${part.weekday} ${part.month} ${part.day} at ${part.hour}:${part.minute} ${timeZone}`;
}

/** Splits a schedule phrase so its clock or interval carries the emphasis. */
function emphasizeTime(phrase: string): AutomationSentenceSegment[] {
  const match = /(\d{2}:\d{2}(?: [A-Za-z_/+-]+)?|midnight(?: [A-Za-z_/+-]+)?|\d+ \w+s\b)/.exec(
    phrase
  );
  if (!match || match.index === undefined) return [plain(phrase)];
  const before = phrase.slice(0, match.index);
  const after = phrase.slice(match.index + match[0].length);
  return [...(before ? [plain(before)] : []), strong(match[0]), ...(after ? [plain(after)] : [])];
}

/**
 * Plain-language cadence: `Every weekday at 09:00 UTC`, `Every 30 minutes`,
 * `Once on Thu Oct 8 at 09:00 UTC`.
 */
export function describeSchedule(
  schedule?: AutomationDraft["schedule"] | AutomationSchedule | null,
  ctx: SentenceContext = {}
): string {
  return scheduleSegments(schedule, ctx)
    .map(segment => segment.text)
    .join("");
}

function scheduleSegments(
  schedule: AutomationDraft["schedule"] | AutomationSchedule | null | undefined,
  ctx: SentenceContext
): AutomationSentenceSegment[] {
  if (!schedule) return [plain("Manual")];
  const days = "days" in schedule ? schedule.days : undefined;
  if (days !== undefined && days.length === 0) {
    return [plain("Every week "), missing("on some days")];
  }
  switch (schedule.mode) {
    case "cron": {
      const human = schedule.expr ? humanCron(schedule.expr) : null;
      if (!human) return [plain("On a custom schedule")];
      const hasClock = /\d{2}:\d{2}|midnight/.test(human);
      return emphasizeTime(`${capitalize(human)}${hasClock ? ` ${zoneLabel(ctx)}` : ""}`);
    }
    case "every":
      return schedule.interval
        ? emphasizeTime(`Every ${humanizeFireWindow(schedule.interval)}`)
        : [plain("Every "), missing("some interval")];
    case "at":
      return schedule.time
        ? [plain("Once on "), strong(formatOnceAt(schedule.time, ctx))]
        : [plain("Once, "), missing("at some time")];
    default:
      return [plain("Manual")];
  }
}

/** Keeps a short single-line prompt as the "to …" clause; long or templated prompts drop it. */
function promptClause(prompt: string | undefined): string | null {
  const text = prompt?.trim() ?? "";
  if (text === "" || text.includes("\n") || text.includes("{{")) return null;
  if (text.length > PROMPT_CLAUSE_MAX) return null;
  const body = text.replace(/[.!]+$/, "");
  if (body === "") return null;
  const lowerFirst = /^[A-Z][a-z]/.test(body) ? body.charAt(0).toLowerCase() + body.slice(1) : body;
  return lowerFirst;
}

function loopBranch(inputs: Readonly<Record<string, unknown>> | undefined): string | null {
  const branch = inputs?.target_branch ?? inputs?.branch;
  return typeof branch === "string" && branch.trim() !== "" ? branch.trim() : null;
}

function doesSegments(target: AutomationDraft["target"]): AutomationSentenceSegment[] {
  switch (target.kind) {
    case "loop": {
      const name = target.loopName?.trim();
      if (!name) return [plain("start the Loop "), missing("a Loop")];
      const branch = loopBranch(target.loopInputs);
      return [
        plain("start the Loop "),
        strong(name),
        ...(branch ? [plain(" on "), strong(branch)] : []),
      ];
    }
    case "task": {
      const title = target.taskTitle?.trim();
      const owner = target.taskOwner?.trim();
      return [
        plain("create a task "),
        title ? strong(title) : missing("with a title"),
        ...(owner ? [plain(" for "), strong(owner)] : []),
      ];
    }
    case "agent": {
      const agent = target.agentName?.trim();
      if (!agent) return [missing("ask an agent")];
      const clause = promptClause(target.prompt);
      return [plain("ask "), strong(agent), ...(clause ? [plain(` to ${clause}`)] : [])];
    }
    default:
      return [missing("do something")];
  }
}

/** `with an error`, `with action deploy on main`, `with stop reason timeout and kind x`. */
function conditionSegments(
  filter: Readonly<Record<string, string>> | null | undefined
): AutomationSentenceSegment[] {
  let branch: string | null = null;
  const clauses: AutomationSentenceSegment[][] = [];
  for (const [rawKey, rawValue] of Object.entries(filter ?? {})) {
    const key = rawKey.trim();
    const value = rawValue.trim();
    if (key === "" || value === "") continue;
    const prose = humanizeFilterKey(key);
    if (prose === "branch" && branch === null) branch = value;
    else if (prose === "stop reason" && value === "error") clauses.push([plain("an error")]);
    else clauses.push([plain(`${prose} `), strong(value)]);
  }
  if (clauses.length === 0 && branch === null) return [];
  const segments: AutomationSentenceSegment[] = [plain(" with ")];
  clauses.forEach((clause, index) => {
    if (index > 0) segments.push(plain(" and "));
    segments.push(...clause);
  });
  if (branch !== null) {
    if (clauses.length === 0) segments.push(plain("branch "), strong(branch));
    else segments.push(plain(" on "), strong(branch));
  }
  return segments;
}

function eventSegments(event: string | undefined): AutomationSentenceSegment[] {
  const id = event?.trim() ?? "";
  if (id === "") return [missing("something happens")];
  const selection = parseEventSelection(id);
  switch (selection.catalogId) {
    case "session.created":
      return [plain("a session starts")];
    case "session.stopped":
      return [plain("a session stops")];
    case "hook.completed":
      return selection.hookName
        ? [plain("the "), strong(selection.hookName), plain(" hook completes")]
        : [missing("a hook"), plain(" completes")];
    case "ext":
      // An editor draft may still miss the extension or its event name.
      return selection.extExt.trim() && selection.extEvent.trim()
        ? [strong(id), plain(" fires")]
        : [missing("an extension event"), plain(" fires")];
    default:
      return [strong(id), plain(" fires")];
  }
}

function projectClause(
  scope: "global" | "workspace" | undefined,
  workspaceId: string | undefined,
  ctx: SentenceContext
): AutomationSentenceSegment[] {
  const id = workspaceId?.trim() ?? "";
  if (scope === "global" || id === "") return [plain(" in any project")];
  const name = ctx.workspaceName?.(id) ?? id;
  return [plain(" in "), strong(name)];
}

function webhookSegments(slug: string | undefined): AutomationSentenceSegment[] {
  const value = slug?.trim();
  return value
    ? [plain("When another app calls the "), strong(value), plain(" link")]
    : [plain("When another app calls "), missing("a link")];
}

interface SentenceSource {
  start: AutomationStart | null;
  schedule?: AutomationDraft["schedule"] | AutomationSchedule | null;
  event?: string;
  filter?: Readonly<Record<string, string>> | null;
  slug?: string;
  scope?: "global" | "workspace";
  workspaceId?: string;
  target: AutomationDraft["target"];
}

function isDraft(
  input: AutomationJob | AutomationTrigger | AutomationDraft
): input is AutomationDraft {
  return "target" in input && "start" in input;
}

function entityTarget(input: AutomationJob | AutomationTrigger): AutomationDraft["target"] {
  const job = isAutomationTrigger(input) ? null : input;
  if (job?.task) {
    return {
      kind: "task",
      // The daemon titles the task with the job name when the title is blank.
      taskTitle: job.task.title?.trim() ? job.task.title : job.name,
      taskOwner: job.task.owner?.ref,
    };
  }
  if (input.target_kind === LOOP_TARGET_KIND) {
    return {
      kind: "loop",
      loopName: input.loop_target?.loop_name,
      loopInputs: input.loop_target?.inputs,
    };
  }
  return { kind: "agent", agentName: input.agent_name, prompt: input.prompt };
}

/** Start kind of a persisted job or trigger. */
export function automationStartOf(input: AutomationJob | AutomationTrigger): AutomationStart {
  if (!isAutomationTrigger(input)) return "schedule";
  return input.event.trim() === "webhook" ? "webhook" : "event";
}

/** Target kind of a persisted job or trigger (`task` = a job with a task target). */
export function automationDoesOf(input: AutomationJob | AutomationTrigger): AutomationDoes {
  const kind = entityTarget(input).kind;
  return kind ?? "agent";
}

function toSource(input: AutomationJob | AutomationTrigger | AutomationDraft): SentenceSource {
  if (isDraft(input)) {
    return {
      start: input.start,
      schedule: input.schedule,
      event: input.event,
      filter: input.filter,
      slug: input.webhook?.slug,
      scope: input.start === "webhook" ? "global" : "workspace",
      workspaceId: input.workspaceId,
      target: input.target,
    };
  }
  if (isAutomationTrigger(input)) {
    return {
      start: automationStartOf(input),
      event: input.event,
      filter: input.filter,
      slug: input.endpoint_slug,
      scope: input.scope,
      workspaceId: input.workspace_id,
      target: entityTarget(input),
    };
  }
  return {
    start: "schedule",
    schedule: input.schedule,
    scope: input.scope,
    workspaceId: input.workspace_id,
    target: entityTarget(input),
  };
}

/** The automation as one sentence of segments. */
export function describeAutomation(
  input: AutomationJob | AutomationTrigger | AutomationDraft,
  ctx: SentenceContext = {}
): AutomationSentence {
  const source = toSource(input);
  const does = doesSegments(source.target);
  switch (source.start) {
    case "schedule":
      return [...scheduleSegments(source.schedule, ctx), plain(", "), ...does, plain(".")];
    case "event":
      return [
        plain("When "),
        ...eventSegments(source.event),
        ...projectClause(source.scope, source.workspaceId, ctx),
        ...conditionSegments(source.filter),
        plain(", "),
        ...does,
        plain("."),
      ];
    case "webhook":
      return [
        ...webhookSegments(source.slug),
        ...conditionSegments(source.filter),
        plain(", "),
        ...does,
        plain("."),
      ];
    default:
      return [plain("When "), missing("it starts"), plain(", "), ...does, plain(".")];
  }
}

/** Plain text of a sentence, for search, aria labels and palette subtitles. */
export function automationSentenceText(sentence: AutomationSentence): string {
  return sentence.map(segment => segment.text).join("");
}

/** True when any part of the sentence is still a placeholder. */
export function automationSentenceIsIncomplete(sentence: AutomationSentence): boolean {
  return sentence.some(segment => segment.missing === true);
}
