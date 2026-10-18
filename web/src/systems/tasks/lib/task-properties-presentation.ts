import { taskHighestAttemptOrdinal } from "./task-command-state";
import { taskPriorityLabel } from "./task-formatters";
import { latestTaskRun } from "./task-run-presentation";
import type { TaskDetailView, TaskExecutionProfile, TaskPriority, TaskRun } from "../types";

type TaskActiveRun = NonNullable<NonNullable<TaskDetailView["summary"]>["active_run"]>;

/** Priority dots are a neutral lightness ramp: hierarchy by weight, never a signal hue. */
const PRIORITY_DOT_CLASS: Record<TaskPriority, string> = {
  urgent: "bg-fg",
  high: "bg-fg-2",
  medium: "bg-subtle",
  low: "bg-line-strong",
};

export function taskPriorityPresentation(priority: TaskPriority): {
  dotClass: string;
  label: string;
} {
  return { dotClass: PRIORITY_DOT_CLASS[priority], label: taskPriorityLabel(priority) };
}

export function taskExecutionProfileSummary(profile?: TaskExecutionProfile | null): {
  worker: string | null;
  model: string | null;
} {
  const worker = profile?.worker;
  const allowedAgents = worker?.allowed_agent_names ?? [];
  const [firstAllowed, ...remainingAllowed] = allowedAgents;

  return {
    worker: worker?.agent_name
      ? worker.agent_name
      : firstAllowed
        ? remainingAllowed.length > 0
          ? `${firstAllowed} +${remainingAllowed.length}`
          : firstAllowed
        : worker?.mode === "inherit"
          ? "Inherited"
          : null,
    model: worker?.model
      ? worker.provider
        ? `${worker.provider} · ${worker.model}`
        : worker.model
      : null,
  };
}

export function taskPropertiesRunSummary(
  detail: TaskDetailView,
  runs: readonly TaskRun[]
): {
  attemptsLabel: string;
  lastFailedRun: TaskRun | null;
  stuckRun: TaskActiveRun | null;
} {
  const record = detail.task;
  const activeRun = detail.summary?.active_run ?? null;
  const highestAttempt = taskHighestAttemptOrdinal(runs, activeRun);
  return {
    attemptsLabel: record.max_attempts
      ? `${highestAttempt} of ${record.max_attempts}`
      : String(highestAttempt),
    lastFailedRun:
      !activeRun && record.status === "failed" ? (latestTaskRun(runs, "failed") ?? null) : null,
    stuckRun: activeRun?.status === "needs_attention" ? activeRun : null,
  };
}
