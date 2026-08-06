import { Link } from "@tanstack/react-router";
import { ChevronRight } from "lucide-react";
import { useState } from "react";

import { MonoId, StatusDot } from "@compozy/ui";
import { cn } from "@/lib/utils";

import {
  formatAttemptLabel,
  formatRelativeTime,
  parseLoopTaskId,
  taskShortId,
  taskStatusLabel,
} from "../lib/task-formatters";
import { summarizeSubtaskSignals, subtaskSummaryLabel } from "../lib/task-hierarchy";
import { resolveTaskListGroupId, listGroupDotProps } from "../lib/task-grouping";
import type { TaskListItem } from "../types";

export interface TaskSubtaskListProps {
  parentTaskId: string;
  subtasks: TaskListItem[];
}

function subtaskDotProps(task: TaskListItem) {
  const groupId = resolveTaskListGroupId(task.status);
  return listGroupDotProps(groupId ?? "queued");
}

function SubtaskRow({ task }: { task: TaskListItem }) {
  const { tone, variant } = subtaskDotProps(task);
  const activeRun = task.active_run ?? null;
  const loopIdentity = parseLoopTaskId(task.id);
  const attemptLabel = activeRun
    ? formatAttemptLabel(activeRun.attempt, loopIdentity ? null : activeRun.max_attempts)
    : null;
  const failedRunError =
    task.status === "failed" && task.active_run?.error ? task.active_run.error : null;

  return (
    <li data-testid={`task-subtask-row-${task.id}`}>
      <Link
        aria-label={`Open subtask ${task.title}`}
        className={cn(
          "flex min-w-0 items-center gap-2.5 rounded-md px-2 py-1.5",
          "hover:bg-row-hover focus-visible:outline-2 focus-visible:outline-offset-1"
        )}
        params={{ id: task.id }}
        to="/tasks/$id"
      >
        <StatusDot label={taskStatusLabel(task.status)} tone={tone} variant={variant} />
        <MonoId data-slot="task-subtask-id" size="sm" value={taskShortId(task)} />
        <span className="min-w-0 flex-1 truncate text-small-body text-muted">
          {taskStatusLabel(task.status)}
          {attemptLabel ? ` · ${attemptLabel}` : ""}
        </span>
        {failedRunError ? (
          <span
            className="min-w-0 max-w-[40%] truncate text-caption text-danger"
            title={failedRunError}
          >
            {failedRunError}
          </span>
        ) : (
          <span className="font-mono text-badge tabular-nums text-faint">
            {formatRelativeTime(task.last_activity_at ?? task.updated_at)}
          </span>
        )}
      </Link>
    </li>
  );
}

/**
 * Collapsed-by-default subtask group rendered inside a parent task row.
 * The summary always carries escalation counts, so collapsing never hides
 * a child that needs someone.
 */
export function TaskSubtaskList({ parentTaskId, subtasks }: TaskSubtaskListProps) {
  const [expanded, setExpanded] = useState(false);
  const summary = summarizeSubtaskSignals(subtasks);
  const hasEscalation = summary.needsAttention > 0 || summary.blocked > 0 || summary.failed > 0;

  return (
    <div
      className="border-t border-line-soft"
      data-slot="task-subtask-list"
      data-testid={`task-subtasks-${parentTaskId}`}
    >
      <button
        aria-expanded={expanded}
        className={cn(
          "flex w-full items-center gap-1.5 px-4 py-2 text-left text-caption",
          "hover:bg-row-hover focus-visible:outline-2 focus-visible:-outline-offset-2",
          hasEscalation ? "text-warning" : "text-muted"
        )}
        data-testid={`task-subtasks-toggle-${parentTaskId}`}
        onClick={() => setExpanded(current => !current)}
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className={cn("size-3 transition-transform duration-fast", expanded && "rotate-90")}
        />
        {subtaskSummaryLabel(summary)}
      </button>
      {expanded ? (
        <ul className="flex flex-col gap-0.5 px-3 pb-2 pl-8" data-slot="task-subtask-rows">
          {subtasks.map(child => (
            <SubtaskRow key={child.id} task={child} />
          ))}
        </ul>
      ) : null}
    </div>
  );
}
