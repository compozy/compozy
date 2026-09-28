import type { TaskListItem } from "../types";
import { formatAttemptLabel, taskOwnerLabel } from "./task-formatters";

/** Labels for the work-item meta line; `null` omits the slot. */
export interface TaskCardMetaModel {
  owner: string;
  attempt: string | null;
  children: string | null;
  dependencies: string | null;
  isSubtask: boolean;
  failedRunError: string | null;
}

function pluralCount(count: number, singular: string, plural: string): string {
  return `${count} ${count === 1 ? singular : plural}`;
}

export function taskCardMetaModel(task: TaskListItem): TaskCardMetaModel {
  const activeRun = task.active_run ?? null;
  const childCount = task.child_count ?? 0;
  const dependencyCount = task.dependency_count ?? 0;
  return {
    owner: taskOwnerLabel(task.owner),
    attempt: activeRun
      ? (formatAttemptLabel(activeRun.attempt, activeRun.max_attempts) ?? "")
      : null,
    children: childCount > 0 ? pluralCount(childCount, "subtask", "subtasks") : null,
    dependencies:
      dependencyCount > 0 ? `Waits on ${pluralCount(dependencyCount, "task", "tasks")}` : null,
    isSubtask: Boolean(task.parent_task_id),
    failedRunError: task.status === "failed" && activeRun?.error ? activeRun.error : null,
  };
}
