import type { ReactElement } from "react";

import {
  Alert,
  EmptyState,
  SectionHeading,
  SkeletonRow,
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
        <Alert data-testid="task-board-error" variant="error">
          {error}
        </Alert>
      ) : null}

      {isLoading ? (
        <div className="grid gap-3 md:grid-cols-3" data-testid="task-board-loading">
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}

      {board ? <CountsSummary counts={board.task_counts} /> : null}

      {!isLoading && board && totalTasks === 0 ? (
        <EmptyState
          data-testid="task-board-empty"
          description="This workflow has no tasks registered with the daemon yet. Sync the workspace from the workflow inventory to pick up task artifacts on disk."
          title="No tasks yet"
        />
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
          className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2"
          data-testid={entry.testId}
          key={entry.label}
        >
          <p className="eyebrow text-muted-foreground">{entry.label}</p>
          <p className="mt-1 font-mono text-lg text-foreground tabular-nums">{entry.value}</p>
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
          <EmptyState
            className="py-5"
            data-testid={`task-board-lane-empty-${lane.status}`}
            description="Tasks will appear here when their status changes."
            title="Lane is empty"
          />
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
      className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2 transition-colors hover:border-border-strong hover:bg-surface-hover"
      data-testid={`task-board-row-${task.task_id}`}
    >
      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3">
        <div className="min-w-0 space-y-1">
          <p className="eyebrow text-muted-foreground">
            #{task.task_number} · {task.type}
          </p>
          <Link
            className="block truncate text-sm font-medium text-foreground hover:underline"
            data-testid={`task-board-link-${task.task_id}`}
            params={{ slug, taskId: task.task_id }}
            to="/workflows/$slug/tasks/$taskId"
            title={task.title}
          >
            {task.title}
          </Link>
          <p className="line-clamp-2 text-xs text-muted-foreground">
            updated {formatTimestamp(task.updated_at)}
            {deps.length > 0 ? ` · depends on ${deps.join(", ")}` : null}
          </p>
        </div>
        <StatusBadge
          data-testid={`task-board-status-${task.task_id}`}
          pulse={tone === "accent"}
          tone={tone}
        >
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
