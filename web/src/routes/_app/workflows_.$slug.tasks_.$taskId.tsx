import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { Alert, SkeletonRow } from "@compozy/ui";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { useRunTranscript } from "@/systems/runs";
import { TaskDetailView, useWorkflowTask, type TaskRelatedRun } from "@/systems/workflows";

export const Route = createFileRoute("/_app/workflows_/$slug/tasks_/$taskId")({
  component: WorkflowTaskDetailRoute,
});

function WorkflowTaskDetailRoute(): ReactElement {
  const { slug, taskId } = useParams({ from: "/_app/workflows_/$slug/tasks_/$taskId" });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const taskQuery = useWorkflowTask(activeWorkspace.id, slug, taskId);
  const transcriptRunId = selectTranscriptRunId(taskQuery.data?.related_runs ?? []);
  const transcriptQuery = useRunTranscript(transcriptRunId);

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={
        <div className="flex w-full items-center justify-between gap-3">
          <button
            className="text-xs font-medium text-primary transition-colors hover:text-foreground"
            data-testid="task-detail-back"
            onClick={() => void navigate({ to: "/workflows/$slug/tasks", params: { slug } })}
            type="button"
          >
            ← Back to {slug} board
          </button>
          <span className="eyebrow text-muted-foreground">task detail</span>
        </div>
      }
    >
      {taskQuery.isLoading && !taskQuery.data ? (
        <div className="space-y-3" data-testid="task-detail-loading">
          <p className="sr-only">Loading task detail…</p>
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {taskQuery.isError && !taskQuery.data ? (
        <Alert data-testid="task-detail-load-error" variant="error">
          {apiErrorMessage(taskQuery.error, `Failed to load task ${taskId} for ${slug}`)}
        </Alert>
      ) : null}
      {taskQuery.data ? (
        <TaskDetailView
          isRefreshing={taskQuery.isRefetching}
          isLoadingRunTranscript={transcriptQuery.isLoading}
          isRunTranscriptError={transcriptQuery.isError}
          payload={taskQuery.data}
          runTranscript={transcriptQuery.data}
          runTranscriptError={
            transcriptQuery.error
              ? apiErrorMessage(transcriptQuery.error, "Failed to load related run transcript")
              : null
          }
          runTranscriptRunId={transcriptRunId}
        />
      ) : null}
    </AppShellLayout>
  );
}

function selectTranscriptRunId(runs: readonly TaskRelatedRun[]): string | null {
  if (runs.length === 0) {
    return null;
  }
  const sorted = [...runs].sort((left, right) => {
    const statusDelta = statusRank(right.status) - statusRank(left.status);
    if (statusDelta !== 0) {
      return statusDelta;
    }
    return timestampValue(right.started_at) - timestampValue(left.started_at);
  });
  return sorted[0]?.run_id ?? null;
}

function statusRank(status: string): number {
  const normalized = status.toLowerCase();
  if (normalized === "running" || normalized === "queued" || normalized === "starting") {
    return 2;
  }
  if (normalized === "completed" || normalized === "succeeded") {
    return 1;
  }
  return 0;
}

function timestampValue(raw: string | undefined): number {
  if (!raw) {
    return 0;
  }
  const timestamp = Date.parse(raw);
  return Number.isNaN(timestamp) ? 0 : timestamp;
}
