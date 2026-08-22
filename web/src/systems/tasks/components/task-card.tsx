import type { ReactNode } from "react";

import { MonoId, Pill } from "@compozy/ui";

import {
  formatAttemptLabel,
  parseLoopTaskId,
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
import { TaskSubtaskList } from "./task-subtask-list";
import { TasksListRow } from "./tasks-list-row";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface TaskCardProps {
  task: TaskListItem;
  /**
   * The profile that owns the task, supplied only in aggregate mode. Distinct
   * from `task.owner`, which names the assignee — two different questions.
   */
  profileOwner?: ProfileOwner;
  /** Children nested under this row; rendered as a collapsed subtask group. */
  subtasks?: TaskListItem[];
}

export function TaskCard({ task, profileOwner, subtasks }: TaskCardProps) {
  const isBlocked = taskIsBlocked(task);
  const needsAttention = task.status === "needs_attention";
  const showApproval = taskHasApprovalPending(task);
  const activeRun = task.active_run ?? null;
  const ownerLabel = taskOwnerLabel(task.owner);
  const nestedSubtasks = subtasks ?? [];
  const childCount = task.child_count ?? 0;
  const dependencyCount = task.dependency_count ?? 0;
  const loopIdentity = parseLoopTaskId(task.id);
  const failedRunError =
    task.status === "failed" && task.active_run?.error ? task.active_run.error : null;

  const metaItems: ReactNode[] = [
    <span data-testid={`task-card-owner-${task.id}`} key="owner">
      {ownerLabel}
    </span>,
  ];
  if (profileOwner) {
    metaItems.push(
      <ProfileOwnerTag
        data-testid={`task-card-profile-${task.id}`}
        key="profile"
        owner={profileOwner}
      />
    );
  }
  if (activeRun) {
    metaItems.push(
      <span data-testid={`task-card-attempt-${task.id}`} key="attempt">
        {/* Loop cells retry through generations; the task-level max is not their budget. */}
        {formatAttemptLabel(activeRun.attempt, loopIdentity ? null : activeRun.max_attempts) ?? ""}
      </span>
    );
  }
  if (childCount > 0 && nestedSubtasks.length === 0) {
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

  if (nestedSubtasks.length === 0) {
    return <TasksListRow meta={metaItems} task={task} trailing={trailing} />;
  }
  return (
    <div
      className="border-b border-line-soft last:border-b-0"
      data-slot="task-card-with-subtasks"
      data-testid={`task-card-nested-${task.id}`}
    >
      <TasksListRow className="border-b-0" meta={metaItems} task={task} trailing={trailing} />
      <TaskSubtaskList parentTaskId={task.id} subtasks={nestedSubtasks} />
    </div>
  );
}
