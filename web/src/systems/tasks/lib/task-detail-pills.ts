import type { PillTone } from "@agh/ui";

import { taskHasApprovalPending, taskWakeIndicatorApplies } from "./task-formatters";
import type { TaskDetailView, TaskPriority } from "../types";

export interface TaskExceptionPill {
  key: string;
  label: string;
  tone: PillTone;
  title?: string;
}

const PRIORITY_EXCEPTION: Partial<Record<TaskPriority, { label: string; tone: PillTone }>> = {
  urgent: { label: "Urgent", tone: "danger" },
  high: { label: "High", tone: "warning" },
  low: { label: "Low", tone: "neutral" },
};

/**
 * Exception-based subhead pills: a pill appears only when it carries
 * information beyond the single head status — priority away from the medium
 * default, a pending approval gate, or a pause that the derived status does
 * not already express. Normal state renders zero pills (quiet page).
 *
 * @see docs/design/opendesign/tasks/TASK-DETAILS-REDESIGN-PLAN.md §4.2
 */
export function projectTaskExceptionPills(detail: TaskDetailView): TaskExceptionPill[] {
  const record = detail.task;
  const pills: TaskExceptionPill[] = [];

  if (taskHasApprovalPending(record)) {
    pills.push({ key: "approval", label: "Approval pending", tone: "info" });
  }

  const priority = record.priority ? PRIORITY_EXCEPTION[record.priority] : undefined;
  if (priority) {
    pills.push({ key: "priority", label: priority.label, tone: priority.tone });
  }

  const isDirectlyPaused = Boolean(record.paused);
  const isEffectivelyPaused = Boolean(detail.summary?.effective_paused ?? record.paused);
  if (isEffectivelyPaused) {
    pills.push({
      key: "paused",
      label: isDirectlyPaused ? "Paused" : "Paused by ancestor",
      tone: "warning",
      title: isDirectlyPaused
        ? record.paused_reason || "New runs stay queued until the task is resumed."
        : detail.summary?.paused_by_task_id
          ? `Paused through ancestor ${detail.summary.paused_by_task_id}.`
          : "Paused through an ancestor task.",
    });
  }

  if (taskWakeIndicatorApplies(record)) {
    const wakeEnabled = record.wake_creator;
    pills.push({
      key: "wake",
      label: wakeEnabled ? "Wake on" : "Wake off",
      tone: wakeEnabled ? "info" : "neutral",
      title: wakeEnabled
        ? "Wake the creating agent session when this task changes."
        : "Do not wake the creating agent session when this task changes.",
    });
  }

  return pills;
}
