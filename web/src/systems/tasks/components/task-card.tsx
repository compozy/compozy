import { Link } from "@tanstack/react-router";

import { Pill, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { taskCardMetaModel } from "../lib/task-card-presentation";
import {
  formatRelativeTime,
  taskApprovalStateLabel,
  taskHasApprovalPending,
  taskPriorityLabel,
  taskStateGlyph,
  taskStatusLabel,
} from "../lib/task-formatters";
import type { TaskListItem } from "../types";
import { TaskLoopRow } from "./task-loop-row";
import { TasksListRow } from "./tasks-list-row";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";
import { WorkspaceBadge, type WorkspaceBadgeIdentity } from "@/systems/workspace";

import { taskWindowBinding } from "../lib/task-detail-search";
import { TaskOwnerMark, TaskOwnerName } from "./task-owner-mark";

export interface TaskCardProps {
  task: TaskListItem;
  onOpenLoopRun?: () => void;
  /**
   * The profile that owns the task, supplied only in aggregate mode. Distinct
   * from `task.owner`, which names the assignee — two different questions.
   */
  profileOwner?: ProfileOwner;
  /**
   * The task's project, supplied only under All projects (a mixed list). The
   * row then opens the task in place, bound to that project.
   */
  projectBadge?: WorkspaceBadgeIdentity;
}

/** One record in the Tasks table: a work item, or a revealed Loop execution record. */
export function TaskCard({ task, onOpenLoopRun, profileOwner, projectBadge }: TaskCardProps) {
  // Loop execution records only reach the listing when the reveal filter is on,
  // and they read by their provenance rather than the work-item meta line.
  if (task.loop) {
    return (
      <TaskLoopRow
        loop={task.loop}
        onOpenRun={onOpenLoopRun}
        profileOwner={profileOwner}
        projectBadge={projectBadge}
        task={task}
      />
    );
  }
  return <TaskWorkItemRow profileOwner={profileOwner} projectBadge={projectBadge} task={task} />;
}

function TaskWorkItemRow({
  task,
  profileOwner,
  projectBadge,
}: {
  task: TaskListItem;
  profileOwner?: ProfileOwner;
  projectBadge?: WorkspaceBadgeIdentity;
}) {
  const meta = taskCardMetaModel(task);
  // `TasksListRow` joins meta children with separators; null slots drop out.
  const metaItems = [
    projectBadge ? (
      <WorkspaceBadge
        className="max-w-40 shrink-[1000]"
        data-testid={`task-card-project-${task.id}`}
        identity={projectBadge}
        key="project"
      />
    ) : null,
    taskHasApprovalPending(task) ? (
      <Pill data-testid={`task-card-approval-${task.id}`} key="approval" size="sm" tone="brand">
        {taskApprovalStateLabel(task.approval_state)}
      </Pill>
    ) : null,
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
        className="min-w-0 truncate text-destructive"
        data-testid={`task-card-error-${task.id}`}
        key="error"
        title={meta.failedRunError}
      >
        {meta.failedRunError}
      </span>
    ) : null,
  ].filter(Boolean);

  return (
    <TasksListRow
      data-status={task.status}
      data-testid={`task-card-${task.id}`}
      id={task.identifier ?? undefined}
      link={
        <Link
          aria-label={`Open ${task.title}`}
          params={{ id: task.id }}
          search={projectBadge ? taskWindowBinding(task.workspace_id) : undefined}
          to="/tasks/$id"
        />
      }
      meta={metaItems.length > 0 ? metaItems : undefined}
      owner={<TaskOwnerChip task={task} />}
      priority={task.priority ? taskPriorityLabel(task.priority) : undefined}
      state={taskStateGlyph(task.status)}
      statusLabel={taskStatusLabel(task.status)}
      title={task.title}
      updated={formatRelativeTime(task.last_activity_at ?? task.updated_at)}
    />
  );
}

/** The assignee as an avatar chip; the name rides in the tooltip and accessible label. */
function TaskOwnerChip({ task }: { task: TaskListItem }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={<span className="inline-flex" data-testid={`task-card-owner-${task.id}`} />}
      >
        <TaskOwnerMark owner={task.owner} />
      </TooltipTrigger>
      <TooltipContent>
        <TaskOwnerName owner={task.owner} />
      </TooltipContent>
    </Tooltip>
  );
}
