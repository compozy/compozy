/**
 * Pure edit rules behind the editor's patch handlers: which switches the form
 * allows, how a schedule changes mode, and the small projections the view reads.
 */

import type { AutomationScheduleMode } from "../types";
import type { AutomationFormDraft } from "./automation-form-draft";
import type { AutomationDoes, AutomationStart } from "./automation-sentence";
import { defaultAtLocal } from "./cron-engine";
import type { WorkspaceOption } from "./trigger-preview";

type JobSchedule = AutomationFormDraft["schedule"];
type JobTask = NonNullable<AutomationFormDraft["task"]>;
type JobOwner = NonNullable<JobTask["owner"]>;

const DEFAULT_CRON_EXPR = "0 9 * * *";
const DEFAULT_EVERY_INTERVAL = "30m";

/**
 * The schedule after a Repeats / Every… / Once switch. The recurring reliability
 * fields ride along so a round trip keeps them; the request normalizer drops
 * them for a one-shot `at`.
 */
export function scheduleForMode(schedule: JobSchedule, next: AutomationScheduleMode): JobSchedule {
  const { catch_up_policy, misfire_grace_seconds } = schedule;
  const recurring = { catch_up_policy, misfire_grace_seconds };
  if (next === "cron")
    return { mode: "cron", expr: schedule.expr || DEFAULT_CRON_EXPR, ...recurring };
  if (next === "every") {
    return { mode: "every", interval: schedule.interval ?? DEFAULT_EVERY_INTERVAL, ...recurring };
  }
  return { mode: "at", time: schedule.time ?? defaultAtLocal(), ...recurring };
}

/** Starts is fixed once created. */
export function canSwitchStart(
  mode: "create" | "edit",
  current: AutomationStart,
  next: AutomationStart
): boolean {
  return mode === "create" && next !== current;
}

/** Does is fixed once created or when a Loop page chose it; tasks are schedules-only. */
export function canSwitchDoes(
  draft: Pick<AutomationFormDraft, "start">,
  context: { mode: "create" | "edit"; lockedLoop?: string; current: AutomationDoes },
  next: AutomationDoes
): boolean {
  if (context.mode === "edit" || context.lockedLoop !== undefined) return false;
  if (next === context.current) return false;
  return next !== "task" || draft.start === "schedule";
}

/** The workspaces the editor can name, falling back to the active one. */
export function resolveWorkspaceOptions(
  workspaces: ReadonlyArray<WorkspaceOption> | undefined,
  activeWorkspaceId: string | null | undefined
): WorkspaceOption[] {
  if (workspaces && workspaces.length > 0) return [...workspaces];
  return activeWorkspaceId ? [{ id: activeWorkspaceId, name: activeWorkspaceId }] : [];
}

/** The project the automation lives in, or `null` for Global. */
export function automationFormDestination(
  scope: AutomationFormDraft["scope"],
  workspaceId: string | undefined,
  workspaceName: (id: string) => string | undefined
): string | null {
  if (scope === "global") return null;
  return workspaceName(workspaceId ?? "") ?? workspaceId ?? "project";
}

/** The one-shot time an edit started from; unchanged, it may already be past. */
export function savedOneShotTime(
  draft: AutomationFormDraft,
  mode: "create" | "edit"
): string | undefined {
  return mode === "edit" && draft.schedule.mode === "at" ? draft.schedule.time : undefined;
}

/** Options opens itself when editing, when retries are on, or when the automation is off. */
export function optionsOpenByDefault(
  draft: AutomationFormDraft,
  mode: "create" | "edit",
  retryStrategy: string
): boolean {
  return mode === "edit" || retryStrategy === "backoff" || draft.enabled === false;
}

/** The task owner for a "For" choice; "Anyone available" has none. */
export function taskOwnerFor(kind: JobOwner["kind"] | "", ref: string): JobOwner | null {
  return kind === "" ? null : { kind, ref };
}
