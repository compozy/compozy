import type { ReactNode } from "react";

import { MonoId, Pill } from "@compozy/ui";

import {
  formatAttemptLabel,
  taskApprovalStateLabel,
  taskHasApprovalPending,
  taskIsBlocked,
  taskOwnerLabel,
  taskPriorityLabel,
  taskPriorityTone,
  taskShortId,
  taskStatusTone,
} from "../lib/task-formatters";
import type { TaskListItem } from "../types";
import { TaskLoopRow } from "./task-loop-row";
import { TasksListRow } from "./tasks-list-row";

export interface TaskCardProps {
  task: TaskListItem;
  onOpenLoopRun?: () => void;
}

export function TaskCard({ task, onOpenLoopRun }: TaskCardProps) {
  // Loop execution records only reach the listing when the reveal filter is on,
  // and they read by their provenance rather than the work-item meta line.
  if (task.loop) {
    return <TaskLoopRow loop={task.loop} onOpenRun={onOpenLoopRun} task={task} />;
  }

  const isBlocked = taskIsBlocked(task);
  const needsAttention = task.status === "needs_attention";
  const showApproval = taskHasApprovalPending(task);
  const activeRun = task.active_run ?? null;
  const ownerLabel = taskOwnerLabel(task.owner);
  const childCount = task.child_count ?? 0;
  const dependencyCount = task.dependency_count ?? 0;
  const failedRunError =
    task.status === "failed" && task.active_run?.error ? task.active_run.error : null;

  const metaItems: ReactNode[] = [
    <span data-testid={`task-card-owner-${task.id}`} key="owner">
      {ownerLabel}
    </span>,
  ];
  if (activeRun) {
    metaItems.push(
      <span data-testid={`task-card-attempt-${task.id}`} key="attempt">
        {formatAttemptLabel(activeRun.attempt, activeRun.max_attempts) ?? ""}
      </span>
    );
  }
  if (childCount > 0) {
    metaItems.push(
      <span data-testid={`task-card-children-${task.id}`} key="children">
        {childCount} {childCount === 1 ? "subtask" : "subtasks"}
      </span>
    );
  }
  if (dependencyCount > 0) {
    metaItems.push(
      <span data-testid={`task-card-deps-${task.id}`} key="deps">
        {dependencyCount} {dependencyCount === 1 ? "dep" : "deps"}
      </span>
    );
  }
  if (task.parent_task_id) {
    metaItems.push(
      <span className="inline-flex items-center gap-1" key="parent">
        <span>parent</span>
        <MonoId size="sm" value={taskShortId({ id: task.parent_task_id })} />
      </span>
    );
  }
  if (failedRunError) {
    metaItems.push(
      <span
        className="min-w-0 truncate text-danger"
        data-testid={`task-card-error-${task.id}`}
        key="error"
        title={failedRunError}
      >
        {failedRunError}
      </span>
    );
  }

  const trailing = (
    <>
      {task.priority ? (
        <Pill size="sm" tone={taskPriorityTone(task.priority)}>
          {taskPriorityLabel(task.priority)}
        </Pill>
      ) : null}
      {showApproval ? (
        <Pill size="sm" tone="accent">
          {taskApprovalStateLabel(task.approval_state)}
        </Pill>
      ) : null}
      {isBlocked ? (
        <Pill
          data-testid={`task-card-blocked-${task.id}`}
          mono
          size="sm"
          tone={taskStatusTone("blocked")}
        >
          Blocked
        </Pill>
      ) : null}
      {needsAttention ? (
        <Pill
          data-testid={`task-card-needs-attention-${task.id}`}
          mono
          size="sm"
          tone={taskStatusTone("needs_attention")}
        >
          Needs attention
        </Pill>
      ) : null}
    </>
  );

  return <TasksListRow meta={metaItems} task={task} trailing={trailing} />;
}
