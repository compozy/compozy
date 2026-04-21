import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { WorkflowSpecView, useWorkflowSpec } from "@/systems/spec";

export const Route = createFileRoute("/_app/workflows_/$slug/spec")({
  component: WorkflowSpecRoute,
});

function WorkflowSpecRoute(): ReactElement {
  const { slug } = useParams({ from: "/_app/workflows_/$slug/spec" });
  const navigate = useNavigate();
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const specQuery = useWorkflowSpec(activeWorkspace.id, slug);

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
      header={
        <div className="flex w-full items-center justify-between gap-3">
          <button
            className="text-xs text-accent hover:underline"
            data-testid="workflow-spec-header-back"
            onClick={() => void navigate({ to: "/workflows" })}
            type="button"
          >
            ← Back to workflows
          </button>
          <span className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
            workflow spec
          </span>
        </div>
      }
    >
      {specQuery.isLoading && !specQuery.data ? (
        <p className="text-sm text-muted-foreground" data-testid="workflow-spec-loading">
          Loading workflow spec…
        </p>
      ) : null}
      {specQuery.isError && !specQuery.data ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="workflow-spec-load-error"
          role="alert"
        >
          {apiErrorMessage(specQuery.error, `Failed to load spec for ${slug}`)}
        </p>
      ) : null}
      {specQuery.data ? (
        <WorkflowSpecView isRefreshing={specQuery.isRefetching} spec={specQuery.data} />
      ) : null}
    </AppShellLayout>
  );
}
