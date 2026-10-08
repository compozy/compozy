/**
 * Readiness of an editor draft (the sentence bar's Ready / Needs a fix and the
 * primary action) plus the Options fold summary. Pure: the form hook feeds the
 * clock, the Loop availability and the cleared-days flag.
 */

import type { AutomationCatchUpPolicy } from "../types";
import {
  automationFormDoes,
  automationFormEvent,
  type AutomationFormDraft,
} from "./automation-form-draft";
import { humanizeFireWindow } from "./automation-formatters";
import { scheduleReadout } from "./automation-form-schedule";
import { parseEventSelection } from "./trigger-event-id";

export interface AutomationFormReadinessContext {
  mode: "create" | "edit";
  now: number;
  daysCleared: boolean;
  loopCompatible: boolean;
}

function locationValid(draft: AutomationFormDraft): boolean {
  return draft.start === "webhook" || draft.scope === "global" || Boolean(draft.workspace_id);
}

function startValid(draft: AutomationFormDraft, ctx: AutomationFormReadinessContext): boolean {
  if (draft.start === "schedule") {
    return scheduleReadout(draft.schedule, ctx.now, { daysCleared: ctx.daysCleared }).valid;
  }
  if (draft.start === "webhook") {
    const hasIds = Boolean(draft.endpoint_slug?.trim()) && Boolean(draft.webhook_id?.trim());
    return ctx.mode === "create" ? hasIds && Boolean(draft.webhook_secret_value?.trim()) : hasIds;
  }
  const event = automationFormEvent(draft);
  const selection = parseEventSelection(event);
  if (selection.catalogId === "" || event !== event.trim()) return false;
  if (selection.family === "hook") {
    return selection.hookName !== "" && selection.hookName === selection.hookName.trim();
  }
  if (selection.family === "ext") {
    return selection.extExt.trim() !== "" && selection.extEvent.trim() !== "";
  }
  return true;
}

/** Index of each condition row whose field or value is still empty. */
export function incompleteConditionRows(filter: AutomationFormDraft["filter"]): Set<number> {
  const rows = new Set<number>();
  Object.entries(filter ?? {}).forEach(([key, value], index) => {
    if (key.trim() === "" || String(value).trim() === "") rows.add(index);
  });
  return rows;
}

function targetValid(draft: AutomationFormDraft, ctx: AutomationFormReadinessContext): boolean {
  const does = automationFormDoes(draft);
  if (does === "task") return true;
  if (does === "loop") {
    const loopWorkspaceId = draft.loop_target?.workspace_id ?? "";
    const loopWorkspaceValid =
      loopWorkspaceId !== "" &&
      (draft.scope === "global" ||
        draft.start === "webhook" ||
        loopWorkspaceId === (draft.workspace_id ?? ""));
    return Boolean(draft.loop_target?.loop_name.trim()) && ctx.loopCompatible && loopWorkspaceValid;
  }
  return draft.agent_name.trim() !== "" && draft.prompt.trim() !== "";
}

/** True when the draft can be saved as written. */
export function automationFormReady(
  draft: AutomationFormDraft,
  ctx: AutomationFormReadinessContext
): boolean {
  const conditionsValid =
    draft.start === "schedule" || incompleteConditionRows(draft.filter).size === 0;
  return (
    draft.name.trim() !== "" &&
    locationValid(draft) &&
    startValid(draft, ctx) &&
    conditionsValid &&
    targetValid(draft, ctx)
  );
}

const MISSED_RUNS_SUMMARY: Record<AutomationCatchUpPolicy, string> = {
  skip_missed: "skip missed",
  coalesce: "catch up once",
  replay: "run every missed",
  run_once_on_catchup: "run once, then continue",
};

/** Options fold summary: `No retries · up to 12/hour · skip missed · on`. */
export function automationOptionsSummary(draft: AutomationFormDraft): string {
  const taskTarget = automationFormDoes(draft) === "task";
  const retry =
    taskTarget || draft.retry?.strategy !== "backoff"
      ? "No retries"
      : `Up to ${draft.retry.max_retries} ${draft.retry.max_retries === 1 ? "retry" : "retries"}`;
  const limit = draft.fire_limit ?? { max: 12, window: "1h" };
  const parts = [retry, `up to ${limit.max}/${humanizeFireWindow(limit.window)}`];
  if (draft.start === "schedule" && draft.schedule.mode !== "at") {
    const policy = draft.schedule.catch_up_policy;
    const loopDefault = automationFormDoes(draft) === "loop" && !policy;
    parts.push(
      loopDefault ? "Loop default for missed" : MISSED_RUNS_SUMMARY[policy ?? "skip_missed"]
    );
  }
  parts.push((draft.enabled ?? true) ? "on" : "off");
  return parts.join(" · ");
}
