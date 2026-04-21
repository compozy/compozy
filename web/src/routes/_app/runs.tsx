import { useState, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import {
  RunsListView,
  useRuns,
  type RunListModeFilter,
  type RunListStatusFilter,
} from "@/systems/runs";

export const Route = createFileRoute("/_app/runs")({
  component: RunsRoute,
});

function RunsRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const [statusFilter, setStatusFilter] = useState<RunListStatusFilter>("all");
  const [modeFilter, setModeFilter] = useState<RunListModeFilter>("all");
  const runsQuery = useRuns({
    workspaceId: activeWorkspace.id,
    status: statusFilter,
    mode: modeFilter,
  });

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
    >
      <RunsListView
        error={runsQuery.isError ? apiErrorMessage(runsQuery.error, "Failed to load runs") : null}
        isLoading={runsQuery.isLoading}
        isRefetching={runsQuery.isRefetching}
        modeFilter={modeFilter}
        onModeChange={setModeFilter}
        onStatusChange={setStatusFilter}
        runs={runsQuery.data ?? []}
        statusFilter={statusFilter}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
