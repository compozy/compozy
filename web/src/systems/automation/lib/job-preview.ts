/**
 * Pure derivation of the Create-Job live-preview view-model from the form draft.
 *
 * Everything the right-hand preview pane shows — the one-line summary, the
 * in-form schedule readout, the live "next runs" list, the run-body digest, and
 * the request payload — is computed here from `draft` alone. No React, no DOM,
 * no side effects: the orchestrator wraps a single call in `useMemo`.
 *
 * Output mode is DERIVED, never stored: the Loop discriminator wins, then a task
 * body selects task mode, otherwise the target is an agent. Every label here is a
 * faithful derivation of the saved schedule/scope/target, never a fabricated metric.
 *
 * `now` defaults to `Date.now()` but is injectable so unit tests stay
 * deterministic and the schedule math is reproducible.
 */

import type { CreateAutomationJobRequest, UpdateAutomationJobRequest } from "../types";
import {
  projectAutomationJobRequest,
  type AutomationEditorMode,
  type AutomationRequestProjection,
} from "./automation-requests";
import { projectAutomationTarget, type LoopTargetProjection } from "./automation-target";
import {
  cronNext,
  formatAbsoluteUtc,
  formatRelative,
  humanCron,
  localInputToDate,
  parseCron,
  parseDuration,
} from "./cron-engine";

export interface JobNextRun {
  index: number;
  relative: string;
  absolute: string;
  isFirst: boolean;
  oneTime: boolean;
}

export interface JobPreviewSummary {
  scheduleLabel: string;
  scopeLabel: string;
  output: "agent" | "loop" | "task";
  agentName: string | null;
  loopTarget: LoopTargetProjection | null;
  ownerLabel: string | null;
}

export interface JobRunDigest {
  output: "agent" | "loop" | "task";
  agentName: string | null;
  loopTarget: LoopTargetProjection | null;
  prompt: string | null;
  task: {
    title: string;
    owner: string;
    participation: {
      channelId: string | null;
      channelStrategy: string | null;
      mode: "local" | "live";
    };
    description: string;
    runStatus: "delegated";
  } | null;
}

export interface JobPreviewModel {
  summary: JobPreviewSummary;
  scheduleValid: boolean;
  scheduleReadout: string;
  nextRuns: JobNextRun[] | null;
  nextRunsEmptyReason: string | null;
  runDigest: JobRunDigest;
  request: AutomationRequestProjection<CreateAutomationJobRequest | UpdateAutomationJobRequest>;
  targetIssue: string | null;
}

type Draft = CreateAutomationJobRequest;
type JobOwner = NonNullable<NonNullable<Draft["task"]>["owner"]>;

const NEXT_RUNS_COUNT = 5;

/** Loop discrimination wins; task mode remains derived from the durable task body. */
function deriveOutput(draft: Draft): "agent" | "loop" | "task" {
  if (projectAutomationTarget(draft).kind === "loop") return "loop";
  return draft.task ? "task" : "agent";
}

function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1);
}

/** Plain-language `kind` / `kind:ref` label for a task owner, or `null`. */
function ownerLabel(owner: JobOwner | null | undefined): string | null {
  if (!owner) {
    return null;
  }
  const ref = owner.ref.trim();
  return ref ? `${owner.kind}:${ref}` : owner.kind;
}

/**
 * Humanized schedule clause for the summary line. UTC is implied by the
 * surrounding copy. Mirrors `schedClause` in the artboard.
 */
function scheduleLabel(draft: Draft, now: number): string {
  const schedule = draft.schedule;
  if (schedule.mode === "cron") {
    const expr = schedule.expr ?? "";
    const human = humanCron(expr);
    return human ? capitalize(human) : `On ${expr}`;
  }
  if (schedule.mode === "every") {
    return `Every ${schedule.interval ?? ""}`;
  }
  const date = localInputToDate(schedule.time ?? "");
  return date ? `Once ${formatRelative(date, now)}` : "Once at a set time";
}

/** Scope clause for the summary line. Mirrors `scopeClause` in the artboard. */
function scopeLabel(draft: Draft): string {
  if (draft.scope === "workspace") {
    return `in ${draft.workspace_id ?? ""}`;
  }
  return "across the whole runtime";
}

function buildSummary(draft: Draft, now: number): JobPreviewSummary {
  const output = deriveOutput(draft);
  const target = projectAutomationTarget(draft);
  return {
    scheduleLabel: scheduleLabel(draft, now),
    scopeLabel: scopeLabel(draft),
    output,
    agentName: output === "agent" ? draft.agent_name : null,
    loopTarget: output === "loop" && target.kind === "loop" ? target : null,
    ownerLabel: output === "task" ? ownerLabel(draft.task?.owner) : null,
  };
}

interface ScheduleReadout {
  valid: boolean;
  text: string;
}

/**
 * In-form schedule validity + readout. Mirrors `renderCronRead`,
 * `renderEveryRead`, and `renderAtRead` from the artboard.
 */
function buildScheduleReadout(draft: Draft, now: number): ScheduleReadout {
  const schedule = draft.schedule;
  if (schedule.mode === "cron") {
    const expr = schedule.expr ?? "";
    if (!parseCron(expr)) {
      return { valid: false, text: "Invalid cron — need 5 fields in range." };
    }
    const human = humanCron(expr);
    return { valid: true, text: `Runs ${human ?? "on this custom schedule"} · UTC` };
  }
  if (schedule.mode === "every") {
    const interval = schedule.interval ?? "";
    if (!parseDuration(interval)) {
      return { valid: false, text: "Invalid duration." };
    }
    return { valid: true, text: `Fires every ${interval}, starting right after creation.` };
  }
  const date = localInputToDate(schedule.time ?? "");
  if (!date) {
    return { valid: false, text: "Pick a date and time." };
  }
  if (date.getTime() <= now) {
    return { valid: false, text: "That time is in the past — the job won't register." };
  }
  return {
    valid: true,
    text: `Runs once, ${formatRelative(date, now)} (${formatAbsoluteUtc(date)} UTC).`,
  };
}

interface NextRunsResult {
  runs: JobNextRun[] | null;
  emptyReason: string | null;
}

function mapNextRuns(dates: Date[], now: number, oneTime: boolean): JobNextRun[] {
  return dates.map((date, index) => ({
    index: index + 1,
    relative: formatRelative(date, now),
    absolute: formatAbsoluteUtc(date),
    isFirst: index === 0,
    oneTime,
  }));
}

/**
 * Live "next runs" preview. Mirrors `renderNextRuns`: `null` when the schedule
 * is invalid (caller shows "fix the schedule"), `[]` with an `emptyReason`
 * string when valid but nothing upcoming.
 */
function buildNextRuns(draft: Draft, now: number): NextRunsResult {
  const schedule = draft.schedule;
  if (schedule.mode === "cron") {
    const dates = cronNext(schedule.expr ?? "", NEXT_RUNS_COUNT, now);
    if (dates === null) {
      return { runs: null, emptyReason: null };
    }
    if (dates.length === 0) {
      return {
        runs: [],
        emptyReason: "No upcoming runs in the next year for this expression.",
      };
    }
    return { runs: mapNextRuns(dates, now, false), emptyReason: null };
  }
  if (schedule.mode === "every") {
    const intervalMs = parseDuration(schedule.interval ?? "");
    if (intervalMs === null) {
      return { runs: null, emptyReason: null };
    }
    const dates: Date[] = [];
    for (let tick = 1; tick <= NEXT_RUNS_COUNT; tick += 1) {
      dates.push(new Date(now + intervalMs * tick));
    }
    return { runs: mapNextRuns(dates, now, false), emptyReason: null };
  }
  const date = localInputToDate(schedule.time ?? "");
  if (!date) {
    return { runs: null, emptyReason: null };
  }
  if (date.getTime() <= now) {
    return {
      runs: [],
      emptyReason: "Won't register — the chosen time has already passed.",
    };
  }
  return { runs: mapNextRuns([date], now, true), emptyReason: null };
}

/**
 * Run-body digest: agent mode renders its prompt, Loop mode renders its typed
 * target, and task mode renders the materialized task with `delegated` status.
 */
function buildRunDigest(draft: Draft): JobRunDigest {
  const output = deriveOutput(draft);
  const target = projectAutomationTarget(draft);
  if (output === "task" && draft.task) {
    const task = draft.task;
    const participation = task.network_participation;
    return {
      output: "task",
      agentName: null,
      loopTarget: null,
      prompt: null,
      task: {
        title: task.title?.trim() ? task.title : draft.name,
        owner: ownerLabel(task.owner) ?? "unassigned",
        participation: {
          mode: participation?.mode === "live" ? "live" : "local",
          channelStrategy: participation?.mode === "live" ? participation.channel_strategy : null,
          channelId:
            participation?.mode === "live" && participation.channel_strategy === "named"
              ? participation.channel_id
              : null,
        },
        description: task.description?.trim() ? task.description : draft.prompt,
        runStatus: "delegated",
      },
    };
  }
  if (output === "loop" && target.kind === "loop") {
    return {
      output: "loop",
      agentName: null,
      loopTarget: target,
      prompt: null,
      task: null,
    };
  }
  return {
    output: "agent",
    agentName: draft.agent_name,
    loopTarget: null,
    prompt: draft.prompt,
    task: null,
  };
}

export function buildJobPreview(
  draft: Draft,
  now: number = Date.now(),
  mode: AutomationEditorMode = "create",
  targetIssue: string | null = null
): JobPreviewModel {
  const readout = buildScheduleReadout(draft, now);
  const nextRuns = buildNextRuns(draft, now);
  return {
    summary: buildSummary(draft, now),
    scheduleValid: readout.valid,
    scheduleReadout: readout.text,
    nextRuns: nextRuns.runs,
    nextRunsEmptyReason: nextRuns.emptyReason,
    runDigest: buildRunDigest(draft),
    request: projectAutomationJobRequest(draft, mode),
    targetIssue,
  };
}
