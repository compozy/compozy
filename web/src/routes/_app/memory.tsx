import type { ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { MemoryIndexView } from "@/systems/memory";
import { useWorkflows } from "@/systems/workflows";

export const Route = createFileRoute("/_app/memory")({
  component: MemoryIndexRoute,
});

function MemoryIndexRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const workflowsQuery = useWorkflows(activeWorkspace.id);

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
    >
      <MemoryIndexView
        error={
          workflowsQuery.isError
            ? apiErrorMessage(workflowsQuery.error, "Failed to load workflows")
            : null
        }
        isLoading={workflowsQuery.isLoading && !workflowsQuery.data}
        isRefetching={workflowsQuery.isRefetching}
        workflows={workflowsQuery.data ?? []}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
