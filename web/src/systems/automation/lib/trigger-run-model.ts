/**
 * Presentation model for one row of a trigger's recent-run accordion.
 *
 * Pure derivation from the persisted run + owning trigger: status tone and
 * label, row icon and meta line, client-side duration (only when both
 * timestamps exist), the expanded drawer sentence, and the gated open link —
 * a link exists only when the daemon recorded the id it points at.
 */

import type { PillTone } from "@compozy/ui";

import { automationStatusTone, formatRunDuration } from "./automation-formatters";
import { projectAutomationTarget } from "./automation-target";
import { triggerTargetName } from "./trigger-sentence";
import type { AutomationRun, AutomationTrigger } from "../types";

export type TriggerRunIcon = "agent" | "agent-off" | "loop" | "clock" | "ban";

export interface TriggerRunMeta {
  text: string;
  /** Mono identifier rendered after the text (session / loop-run id). */
  monoId: string | null;
}

export interface TriggerRunLink {
  kind: "session" | "loop-run";
  id: string;
  label: "Open session" | "Open loop run";
}

export interface TriggerRunDrawerLine {
  text: string;
  tone: "default" | "danger";
}

export interface TriggerRunView {
  id: string;
  statusLabel: string;
  tone: PillTone;
  pulse: boolean;
  icon: TriggerRunIcon;
  meta: TriggerRunMeta;
  /** Client-side duration; `—` until both `started_at` and `ended_at` exist. */
  duration: string;
  drawerLines: TriggerRunDrawerLine[];
  /** Present only when the daemon recorded the target id — never a disabled stub. */
  link: TriggerRunLink | null;
}

function statusLabel(status: AutomationRun["status"]): string {
  return `${status.charAt(0).toUpperCase()}${status.slice(1)}`;
}

function runIcon(run: AutomationRun, isLoopTarget: boolean): TriggerRunIcon {
  switch (run.status) {
    case "scheduled":
      return "clock";
    case "canceled":
      return "ban";
    case "failed":
      return "agent-off";
    default:
      return isLoopTarget || run.loop_run_id ? "loop" : "agent";
  }
}

function firstLine(text: string): string {
  return text.split("\n", 1)[0].trim();
}

function runMeta(run: AutomationRun): TriggerRunMeta {
  if (run.status === "failed") {
    const diagnostic = [run.error, run.delivery_error].find(value => value?.trim());
    if (diagnostic) return { text: firstLine(diagnostic), monoId: null };
  }
  if (run.loop_run_id) return { text: "Loop run", monoId: run.loop_run_id };
  if (run.session_id) return { text: "Session", monoId: run.session_id };
  if (run.status === "scheduled") return { text: "Accepted · not started", monoId: null };
  if (run.status === "canceled") return { text: "Canceled", monoId: null };
  return { text: statusLabel(run.status), monoId: null };
}

function runLink(run: AutomationRun): TriggerRunLink | null {
  if (run.loop_run_id) {
    return { kind: "loop-run", id: run.loop_run_id, label: "Open loop run" };
  }
  if (run.session_id) {
    return { kind: "session", id: run.session_id, label: "Open session" };
  }
  return null;
}

function deliveryId(run: AutomationRun): string | null {
  const value = run.metadata?.["delivery_id"];
  return typeof value === "string" && value.trim() !== "" ? value : null;
}

function drawerLines(
  run: AutomationRun,
  trigger: AutomationTrigger,
  isLoopTarget: boolean
): TriggerRunDrawerLine[] {
  const targetName = triggerTargetName(trigger);
  const noun = trigger.event === "webhook" ? "delivery" : "event";
  switch (run.status) {
    case "completed": {
      if (isLoopTarget || run.loop_run_id) {
        return [
          {
            text: `${targetName} finished. Open the loop run for the attempt ledger.`,
            tone: "default",
          },
        ];
      }
      const delivery = deliveryId(run);
      return [
        {
          text: delivery
            ? `${targetName} finished. Delivery ${delivery}.`
            : `${targetName} finished. Open the session to read the summary.`,
          tone: "default",
        },
      ];
    }
    case "delegated":
      return [
        { text: `Handed to ${targetName}. The loop owns the rest of the run.`, tone: "default" },
      ];
    case "running":
      return [{ text: `${targetName} is still working on this ${noun}.`, tone: "default" }];
    case "failed": {
      const lines: TriggerRunDrawerLine[] = [];
      if (run.error?.trim()) lines.push({ text: run.error, tone: "danger" });
      if (run.delivery_error?.trim()) {
        lines.push({ text: `Delivery: ${run.delivery_error}`, tone: "danger" });
      }
      if (lines.length === 0) {
        lines.push({ text: "The run failed before any detail was recorded.", tone: "danger" });
      }
      if (run.attempt > 1) lines.push({ text: `Attempt ${run.attempt}.`, tone: "default" });
      return lines;
    }
    case "scheduled": {
      const destination = isLoopTarget ? "loop run" : "session";
      return [
        {
          text: `The ${noun} was accepted. No ${destination} yet — the open link appears once the daemon records one.`,
          tone: "default",
        },
      ];
    }
    default:
      return [{ text: "This run was canceled.", tone: "default" }];
  }
}

export function buildTriggerRunView(
  run: AutomationRun,
  trigger: AutomationTrigger
): TriggerRunView {
  const isLoopTarget = projectAutomationTarget(trigger).kind === "loop";
  const hasBothTimestamps = Boolean(run.started_at) && Boolean(run.ended_at);
  return {
    id: run.id,
    statusLabel: statusLabel(run.status),
    tone: automationStatusTone(run.status),
    pulse: run.status === "running",
    icon: runIcon(run, isLoopTarget),
    meta: runMeta(run),
    duration: hasBothTimestamps ? formatRunDuration(run) : "—",
    drawerLines: drawerLines(run, trigger, isLoopTarget),
    link: runLink(run),
  };
}
