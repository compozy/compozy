import { Link } from "@tanstack/react-router";

import { Pill } from "@compozy/ui";

import {
  RUN_GONE_LABEL,
  taskLoopIdentityLabel,
  taskLoopRoleLabel,
  taskLoopRunLink,
  type TaskLoopProvenance,
} from "../lib/task-loop-identity";
import { formatRelativeTime, taskStateGlyph, taskStatusLabel } from "../lib/task-formatters";
import type { TaskListItem } from "../types";
import { TasksListRow } from "./tasks-list-row";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface TaskLoopRowProps {
  task: TaskListItem;
  loop: TaskLoopProvenance;
  onOpenRun?: () => void;
  profileOwner?: ProfileOwner;
}

/**
 * A revealed Loop execution record in the Tasks table (US-002.AC-1).
 *
 * Exclusion is default-filtering, never erasure (ADR-001), so a revealed record
 * keeps the work item's row geometry and earns its distinction from two separate
 * channels: structure — the neutral role tag after its name — and status — the
 * state glyph column. Identity is plain words end to end; the machine run id only
 * ever appears in the identifier column.
 */
export function TaskLoopRow({ task, loop, onOpenRun, profileOwner }: TaskLoopRowProps) {
  const runLink = taskLoopRunLink(loop);
  const identity = taskLoopIdentityLabel(loop);
  const meta = [
    <Pill data-slot="task-loop-row-role" key="role" size="sm" tone="neutral">
      {taskLoopRoleLabel(loop)}
    </Pill>,
    profileOwner ? <ProfileOwnerTag key="profile" owner={profileOwner} /> : null,
  ];

  // Retention removed the run: the record stays and says so, with the id
  // carrying identity. A dead link would be worse than no link (US-002.EC-2).
  if (!runLink) {
    return (
      <TasksListRow
        data-loop-role={loop.role}
        data-slot="task-loop-row"
        data-testid={`task-loop-row-${task.id}`}
        id={<span data-slot="task-loop-row-run-id">{loop.run_id}</span>}
        link={null}
        meta={meta}
        state="stopped"
        statusLabel={
          <span data-testid={`task-loop-row-run-gone-${task.id}`}>{RUN_GONE_LABEL}</span>
        }
        title={<span data-slot="task-loop-row-identity">{identity}</span>}
      />
    );
  }

  return (
    <TasksListRow
      data-loop-role={loop.role}
      data-slot="task-loop-row"
      data-status={task.status}
      data-testid={`task-loop-row-${task.id}`}
      link={
        <Link
          aria-label={`Open run for ${identity}`}
          onClick={onOpenRun}
          params={runLink.params}
          to={runLink.to}
        />
      }
      meta={meta}
      state={taskStateGlyph(task.status)}
      statusLabel={taskStatusLabel(task.status)}
      title={<span data-slot="task-loop-row-identity">{identity}</span>}
      updated={formatRelativeTime(task.last_activity_at ?? task.updated_at)}
    />
  );
}
