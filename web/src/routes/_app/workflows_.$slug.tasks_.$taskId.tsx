import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { TaskDetailView, useWorkflowTask } from "@/systems/workflows";

export const Route = createFileRoute("/_app/workflows_/$slug/tasks_/$taskId")({
  component: WorkflowTaskDetailRoute,
});

function WorkflowTaskDetailRoute(): ReactElement {
  const { slug, taskId } = useParams({ from: "/_app/workflows_/$slug/tasks_/$taskId" });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const taskQuery = useWorkflowTask(activeWorkspace.id, slug, taskId);

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={
        <div className="flex w-full items-center justify-between gap-3">
          <button
            className="text-xs text-accent hover:underline"
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
        <p className="text-sm text-muted-foreground" data-testid="task-detail-loading">
          Loading task detail…
        </p>
      ) : null}
      {taskQuery.isError && !taskQuery.data ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="task-detail-load-error"
          role="alert"
        >
          {apiErrorMessage(taskQuery.error, `Failed to load task ${taskId} for ${slug}`)}
        </p>
      ) : null}
      {taskQuery.data ? (
        <TaskDetailView isRefreshing={taskQuery.isRefetching} payload={taskQuery.data} />
      ) : null}
    </AppShellLayout>
  );
}
