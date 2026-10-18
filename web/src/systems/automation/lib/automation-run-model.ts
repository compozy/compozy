/**
 * Presentation model for one row of an automation's run accordion, for jobs
 * and triggers alike.
 *
 * Pure derivation from the persisted run + owning automation: the shared
 * formatter word and `StateGlyph`, the row's meta (what it produced or why it
 * stopped, always neutral), a client-side duration, the drawer lines (danger
 * only for a failure's cause), and the open link — present only when the
 * daemon recorded the id it points at.
 */

import type { StateGlyphState } from "@compozy/ui";

import {
  automationLastRunLabel,
  automationRunSkipReason,
  automationRunStateGlyph,
  formatRunDuration,
  type AutomationSkipReason,
} from "./automation-formatters";
import { isAutomationTrigger, type AutomationEntity } from "./automation-entity";
import { automationDoesOf } from "./automation-sentence";
import { projectAutomationTarget } from "./automation-target";
import type { AutomationRun } from "../types";

export type AutomationRunIcon = "agent" | "session" | "loop" | "task" | "manual" | "skip" | "clock";

export interface AutomationRunMeta {
  text: string;
  /** Mono identifier rendered after the text (session / loop-run / task id). */
  monoId: string | null;
}

export interface AutomationRunLink {
  kind: "session" | "loop-run" | "task";
  id: string;
  label: "Open session" | "Open loop run" | "Open task";
  workspaceId?: string;
}

export interface AutomationRunDrawerLine {
  id: "summary" | "error" | "delivery-error" | "attempt" | "retries";
  text: string;
  tone: "default" | "danger";
}

export interface AutomationRunView {
  id: string;
  statusLabel: string;
  glyph: StateGlyphState;
  icon: AutomationRunIcon;
  meta: AutomationRunMeta;
  /** Client-side duration; `—` until both `started_at` and `ended_at` exist. */
  duration: string;
  /** Start time, or the scheduled time for a reservation that has not begun. */
  at: string | null;
  drawerLines: AutomationRunDrawerLine[];
  link: AutomationRunLink | null;
  /** A failed run of an automation with no retries offers the fix in its drawer. */
  offerRetries: boolean;
}

const SKIP_TEXT = {
  self_overlap: "The previous run was still going",
  misfire_grace_exceeded: "CompozyOS was off at the start time",
} as const satisfies Record<AutomationSkipReason, string>;

function firstLine(text: string): string {
  return text.split("\n", 1)[0].trim();
}

/**
 * A Run now run is a job run the scheduler never reserved: manual dispatch
 * records neither a fire id nor a scheduled time (`DispatchKindManual`).
 */
function isManualRun(run: AutomationRun): boolean {
  return Boolean(run.job_id) && !run.fire_id && !run.scheduled_at;
}

function targetName(entity: AutomationEntity): string {
  if (automationDoesOf(entity) === "task") return "The task";
  const target = projectAutomationTarget(entity);
  return target.kind === "loop" ? target.loopName : target.agentName;
}

function produced(run: AutomationRun): AutomationRunMeta & { icon: AutomationRunIcon } {
  if (run.loop_run_id) return { icon: "loop", text: "Loop run", monoId: run.loop_run_id };
  if (run.task_id) return { icon: "task", text: "Task", monoId: run.task_id };
  if (run.session_id) return { icon: "session", text: "Session", monoId: run.session_id };
  return { icon: "clock", text: "Accepted · not started yet", monoId: null };
}

function rowMeta(
  run: AutomationRun,
  skip: AutomationSkipReason | null
): AutomationRunMeta & { icon: AutomationRunIcon } {
  if (skip) return { icon: "skip", text: SKIP_TEXT[skip], monoId: null };
  if (run.status === "failed") {
    const cause = [run.error, run.delivery_error].find(value => value?.trim());
    if (cause) return { icon: "agent", text: firstLine(cause), monoId: null };
  }
  if (run.status === "canceled")
    return { icon: "skip", text: "This run was canceled", monoId: null };
  const output = produced(run);
  if (isManualRun(run) && output.monoId) {
    return { ...output, icon: "manual", text: `Run now · ${output.text}` };
  }
  return output;
}

function runLink(
  run: AutomationRun,
  loopWorkspaceId: string | undefined
): AutomationRunLink | null {
  if (run.loop_run_id) {
    return {
      kind: "loop-run",
      id: run.loop_run_id,
      label: "Open loop run",
      ...(loopWorkspaceId ? { workspaceId: loopWorkspaceId } : {}),
    };
  }
  if (run.task_id) return { kind: "task", id: run.task_id, label: "Open task" };
  if (run.session_id) return { kind: "session", id: run.session_id, label: "Open session" };
  return null;
}

function failedLines(run: AutomationRun, noRetries: boolean): AutomationRunDrawerLine[] {
  const lines: AutomationRunDrawerLine[] = [];
  if (run.error?.trim()) lines.push({ id: "error", text: run.error, tone: "danger" });
  if (run.delivery_error?.trim()) {
    lines.push({ id: "delivery-error", text: `Delivery: ${run.delivery_error}`, tone: "danger" });
  }
  if (lines.length === 0) {
    lines.push({
      id: "error",
      text: "The run failed before any detail was recorded.",
      tone: "danger",
    });
  }
  if (run.attempt > 1) {
    lines.push({ id: "attempt", text: `Attempt ${run.attempt}.`, tone: "default" });
  }
  if (noRetries) {
    lines.push({ id: "retries", text: "No retries are set, so it stopped here.", tone: "default" });
  }
  return lines;
}

function summary(text: string): AutomationRunDrawerLine[] {
  return [{ id: "summary", text, tone: "default" }];
}

function drawerLines(
  run: AutomationRun,
  entity: AutomationEntity,
  skip: AutomationSkipReason | null
): AutomationRunDrawerLine[] {
  const name = targetName(entity);
  const noun = isAutomationTrigger(entity)
    ? entity.event === "webhook"
      ? "request"
      : "event"
    : "run";
  const byHand = isManualRun(run) ? "Started by hand with Run now. " : "";
  switch (run.status) {
    case "completed":
      return summary(
        run.loop_run_id
          ? `${byHand}${name} finished. Open the loop run for the attempt ledger.`
          : `${byHand}${name} finished. Open the session to read the summary.`
      );
    case "delegated":
      return summary(
        run.task_id || automationDoesOf(entity) === "task"
          ? `${byHand}Created a task. The task owns the rest of the run.`
          : `${byHand}Handed to ${name}. The Loop owns the rest of the run.`
      );
    case "running":
      return summary(`${byHand}${name} is still working on this ${noun}.`);
    case "failed":
      return failedLines(run, entity.retry.strategy === "none");
    case "scheduled":
      return summary(`The ${noun} was accepted. The open link appears once it starts.`);
    case "canceled":
      return summary(skip ? `${SKIP_TEXT[skip]}.` : "This run was canceled.");
  }
}

export function buildAutomationRunView(
  run: AutomationRun,
  entity: AutomationEntity
): AutomationRunView {
  const target = projectAutomationTarget(entity);
  const skip = automationRunSkipReason(run);
  const meta = rowMeta(run, skip);
  const hasBothTimestamps = Boolean(run.started_at) && Boolean(run.ended_at);
  return {
    id: run.id,
    statusLabel: automationLastRunLabel({
      status: run.status,
      ...(skip ? { skipReason: skip } : {}),
    }),
    glyph: automationRunStateGlyph(run.status),
    icon: meta.icon,
    meta: { text: meta.text, monoId: meta.monoId },
    duration: hasBothTimestamps ? formatRunDuration(run) : "—",
    at: run.started_at ?? run.scheduled_at ?? null,
    drawerLines: drawerLines(run, entity, skip),
    link: runLink(run, target.kind === "loop" ? target.workspaceId : undefined),
    offerRetries: run.status === "failed" && entity.retry.strategy === "none",
  };
}
