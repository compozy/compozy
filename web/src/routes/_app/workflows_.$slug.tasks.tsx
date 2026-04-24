import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { TaskBoardView, useWorkflowBoard } from "@/systems/workflows";

export const Route = createFileRoute("/_app/workflows_/$slug/tasks")({
  component: WorkflowTasksBoardRoute,
});

function WorkflowTasksBoardRoute(): ReactElement {
  const { slug } = useParams({ from: "/_app/workflows_/$slug/tasks" });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const boardQuery = useWorkflowBoard(activeWorkspace.id, slug);

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
        workflowSlug={slug}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
