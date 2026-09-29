import { Pill } from "@compozy/ui";

import { taskCardMetaModel } from "../lib/task-card-presentation";
import {
  taskApprovalStateLabel,
  taskHasApprovalPending,
  taskPriorityLabel,
  taskPriorityTone,
} from "../lib/task-formatters";
import type { TaskListItem } from "../types";
import { TaskLoopRow } from "./task-loop-row";
import { TasksListRow } from "./tasks-list-row";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface TaskCardProps {
  task: TaskListItem;
  onOpenLoopRun?: () => void;
  /**
   * The profile that owns the task, supplied only in aggregate mode. Distinct
   * from `task.owner`, which names the assignee — two different questions.
   */
  profileOwner?: ProfileOwner;
}

export function TaskCard({ task, onOpenLoopRun, profileOwner }: TaskCardProps) {
  // Loop execution records only reach the listing when the reveal filter is on,
  // and they read by their provenance rather than the work-item meta line.
  if (task.loop) {
    return (
      <TaskLoopRow
        loop={task.loop}
        onOpenRun={onOpenLoopRun}
        profileOwner={profileOwner}
        task={task}
      />
    );
  }
  return <TaskWorkItemCard profileOwner={profileOwner} task={task} />;
}

function TaskWorkItemCard({
  task,
  profileOwner,
}: {
  task: TaskListItem;
  profileOwner?: ProfileOwner;
}) {
  const meta = taskCardMetaModel(task);
  // `TasksListRow` joins meta children with separators; null slots drop out.
  const metaItems = [
    <span data-testid={`task-card-owner-${task.id}`} key="owner">
      {meta.owner}
    </span>,
    profileOwner ? (
      <ProfileOwnerTag
        data-testid={`task-card-profile-${task.id}`}
        key="profile"
        owner={profileOwner}
      />
    ) : null,
    meta.attempt === null ? null : (
      <span data-testid={`task-card-attempt-${task.id}`} key="attempt">
        {meta.attempt}
      </span>
    ),
    meta.children ? (
      <span data-testid={`task-card-children-${task.id}`} key="children">
        {meta.children}
      </span>
    ) : null,
    meta.dependencies ? (
      <span data-testid={`task-card-deps-${task.id}`} key="deps">
        {meta.dependencies}
      </span>
    ) : null,
    meta.isSubtask ? (
      <span data-testid={`task-card-subtask-${task.id}`} key="parent">
        Subtask
      </span>
    ) : null,
    meta.failedRunError ? (
      <span
        className="min-w-0 truncate text-danger"
        data-testid={`task-card-error-${task.id}`}
        key="error"
        title={meta.failedRunError}
      >
        {meta.failedRunError}
      </span>
    ) : null,
  ];
  return <TasksListRow meta={metaItems} task={task} trailing={<TaskCardTrailing task={task} />} />;
}

function TaskCardTrailing({ task }: { task: TaskListItem }) {
  return (
    <>
      {task.priority ? (
        <Pill size="sm" tone={taskPriorityTone(task.priority)}>
          {taskPriorityLabel(task.priority)}
        </Pill>
      ) : null}
      {taskHasApprovalPending(task) ? (
        <Pill size="sm" tone="accent">
          {taskApprovalStateLabel(task.approval_state)}
        </Pill>
      ) : null}
    </>
  );
}
