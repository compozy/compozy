import type { ReactElement } from "react";

import { createFileRoute, useNavigate, useParams } from "@tanstack/react-router";
import { Alert, SkeletonRow } from "@compozy/ui";

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
            className="text-xs font-medium text-primary transition-colors hover:text-foreground"
            data-testid="workflow-spec-header-back"
            onClick={() => void navigate({ to: "/workflows" })}
            type="button"
          >
            ← Back to workflows
          </button>
          <span className="eyebrow text-muted-foreground">workflow spec</span>
        </div>
      }
    >
      {specQuery.isLoading && !specQuery.data ? (
        <div className="space-y-3" data-testid="workflow-spec-loading">
          <p className="sr-only">Loading workflow spec…</p>
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {specQuery.isError && !specQuery.data ? (
        <Alert data-testid="workflow-spec-load-error" variant="error">
          {apiErrorMessage(specQuery.error, `Failed to load spec for ${slug}`)}
        </Alert>
      ) : null}
      {specQuery.data ? (
        <WorkflowSpecView isRefreshing={specQuery.isRefetching} spec={specQuery.data} />
      ) : null}
    </AppShellLayout>
  );
}
