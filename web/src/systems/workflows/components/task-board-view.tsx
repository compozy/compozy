import type { ReactElement } from "react";

import {
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
  type StatusBadgeTone,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { TaskBoardPayload, TaskCard, TaskLane, WorkflowTaskCounts } from "../types";

export interface TaskBoardViewProps {
  board?: TaskBoardPayload;
  isLoading: boolean;
  isRefetching: boolean;
  error?: string | null;
  workflowSlug: string;
  workspaceName: string;
}

export function TaskBoardView(props: TaskBoardViewProps): ReactElement {
  const { board, isLoading, isRefetching, error, workflowSlug, workspaceName } = props;
  const lanes = board?.lanes ?? [];
  const totalTasks = board?.task_counts?.total ?? 0;

  return (
    <div className="space-y-6" data-testid="task-board-view">
      <SectionHeading
        description={`Tasks registered for ${workflowSlug} in ${workspaceName}.`}
        eyebrow="Workflow · Tasks"
        title={workflowSlug}
      />

      {error ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="task-board-error"
          role="alert"
        >
          {error}
        </p>
      ) : null}

      {isLoading ? (
        <p className="text-sm text-muted-foreground" data-testid="task-board-loading">
          Loading task board…
        </p>
      ) : null}

      {board ? <CountsSummary counts={board.task_counts} /> : null}

      {!isLoading && board && totalTasks === 0 ? (
        <SurfaceCard data-testid="task-board-empty">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>Empty</SurfaceCardEyebrow>
              <SurfaceCardTitle>No tasks yet</SurfaceCardTitle>
              <SurfaceCardDescription>
                This workflow does not have any tasks registered with the daemon yet. Sync the
                workspace from the workflow inventory to pick up task artifacts on disk.
              </SurfaceCardDescription>
            </div>
          </SurfaceCardHeader>
        </SurfaceCard>
      ) : null}

      {lanes.length > 0 ? (
        <div className="grid gap-4 lg:grid-cols-2 xl:grid-cols-3" data-testid="task-board-lanes">
          {lanes.map(lane => (
            <BoardLane key={`${lane.status}-${lane.title}`} lane={lane} slug={workflowSlug} />
          ))}
        </div>
      ) : null}

      {isRefetching ? (
        <p className="text-xs text-muted-foreground" data-testid="task-board-refreshing">
          refreshing…
        </p>
      ) : null}
    </div>
  );
}

function CountsSummary({ counts }: { counts: WorkflowTaskCounts }): ReactElement {
  const entries: { label: string; value: number; testId: string }[] = [
    { label: "Total", value: counts.total, testId: "task-board-count-total" },
    { label: "Completed", value: counts.completed, testId: "task-board-count-completed" },
    { label: "Pending", value: counts.pending, testId: "task-board-count-pending" },
  ];
  return (
    <div className="grid gap-3 sm:grid-cols-3" data-testid="task-board-counts">
      {entries.map(entry => (
        <div
          className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
          data-testid={entry.testId}
          key={entry.label}
        >
          <p className="eyebrow text-muted-foreground">{entry.label}</p>
          <p className="mt-1 font-display text-lg tracking-[-0.02em] text-foreground">
            {entry.value}
          </p>
        </div>
      ))}
    </div>
  );
}

function BoardLane({ lane, slug }: { lane: TaskLane; slug: string }): ReactElement {
  const items = lane.items ?? [];
  const tone = resolveLaneTone(lane.status);
  return (
    <SurfaceCard data-testid={`task-board-lane-${lane.status}`}>
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>{lane.status}</SurfaceCardEyebrow>
          <SurfaceCardTitle>{lane.title}</SurfaceCardTitle>
          <SurfaceCardDescription>
            {items.length === 0
              ? "No tasks in this lane."
              : `${items.length} task${items.length === 1 ? "" : "s"} in this lane.`}
          </SurfaceCardDescription>
        </div>
        <StatusBadge data-testid={`task-board-lane-count-${lane.status}`} tone={tone}>
          {items.length}
        </StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {items.length === 0 ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid={`task-board-lane-empty-${lane.status}`}
          >
            Lane is empty.
          </p>
        ) : (
          <ul className="space-y-2" data-testid={`task-board-lane-items-${lane.status}`}>
            {items.map(task => (
              <TaskRow key={task.task_id} slug={slug} task={task} />
            ))}
          </ul>
        )}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function TaskRow({ slug, task }: { slug: string; task: TaskCard }): ReactElement {
  const tone = resolveStatusTone(task.status);
  const deps = task.depends_on ?? [];
  return (
    <li
      className="rounded-[var(--radius-md)] border border-border bg-black/10 px-3 py-2"
      data-testid={`task-board-row-${task.task_id}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="eyebrow text-muted-foreground">
            #{task.task_number} · {task.type}
          </p>
          <Link
            className="truncate text-sm font-medium text-foreground hover:underline"
            data-testid={`task-board-link-${task.task_id}`}
            params={{ slug, taskId: task.task_id }}
            to="/workflows/$slug/tasks/$taskId"
          >
            {task.title}
          </Link>
          <p className="text-xs text-muted-foreground">
            updated {formatTimestamp(task.updated_at)}
            {deps.length > 0 ? ` · depends on ${deps.join(", ")}` : null}
          </p>
        </div>
        <StatusBadge data-testid={`task-board-status-${task.task_id}`} tone={tone}>
          {task.status}
        </StatusBadge>
      </div>
    </li>
  );
}

export function resolveStatusTone(status: string): StatusBadgeTone {
  const normalized = status.trim().toLowerCase();
  switch (normalized) {
    case "completed":
    case "done":
      return "success";
    case "in_progress":
    case "in-progress":
    case "running":
      return "accent";
    case "blocked":
    case "failed":
      return "danger";
    case "review":
    case "needs_review":
      return "warning";
    case "pending":
    case "todo":
      return "info";
    default:
      return "neutral";
  }
}

function resolveLaneTone(status: string): StatusBadgeTone {
  return resolveStatusTone(status);
}

function formatTimestamp(raw: string | undefined): string {
  if (!raw) {
    return "unknown";
  }
  try {
    return new Date(raw).toLocaleString();
  } catch {
    return raw;
  }
}
