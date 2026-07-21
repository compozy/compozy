import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { taskGroupSearchSchema } from "@/lib/task-group-search";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { TaskBoardView, useWorkflowBoard } from "@/systems/workflows";

export const Route = createFileRoute("/_app/workflows_/$slug/tasks")({
  component: WorkflowTasksBoardRoute,
  validateSearch: taskGroupSearchSchema,
});

function WorkflowTasksBoardRoute(): ReactElement {
  const { slug } = useParams({ from: "/_app/workflows_/$slug/tasks" });
  const { task_group_id: taskGroupId } = Route.useSearch();
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const boardQuery = useWorkflowBoard(activeWorkspace.id, slug, taskGroupId);

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={
        <div className="flex w-full items-center justify-between gap-3">
          <button
            className="text-xs font-medium text-primary transition-colors hover:text-foreground"
            data-testid="task-board-back"
            onClick={() => void navigate({ to: "/workflows" })}
            type="button"
          >
            ← Back to workflows
          </button>
          <span className="eyebrow text-muted-foreground">task board</span>
        </div>
      }
    >
      <TaskBoardView
        board={boardQuery.data}
        error={
          boardQuery.isError
            ? apiErrorMessage(boardQuery.error, `Failed to load task board for ${slug}`)
            : null
        }
        isLoading={boardQuery.isLoading}
        isRefetching={boardQuery.isRefetching}
        taskGroupId={taskGroupId}
        workflowSlug={slug}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
