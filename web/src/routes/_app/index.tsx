import { useState, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";
import { Alert, SkeletonRow } from "@compozy/ui";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { DashboardView, useDashboard } from "@/systems/dashboard";
import { formatWorkflowSyncResult, useSyncWorkflows } from "@/systems/workflows";

export const Route = createFileRoute("/_app/")({
  component: DashboardRoute,
});

function DashboardRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const dashboardQuery = useDashboard(activeWorkspace.id);
  const syncAll = useSyncWorkflows();
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  async function handleSyncAll() {
    setActionMessage(null);
    setActionError(null);
    try {
      const result = await syncAll.mutateAsync({ workspaceId: activeWorkspace.id });
      setActionMessage(formatWorkflowSyncResult(result));
    } catch (error) {
      setActionError(apiErrorMessage(error, "Sync failed"));
    }
  }

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
    >
      {dashboardQuery.isLoading && !dashboardQuery.data ? (
        <div className="space-y-3" data-testid="dashboard-loading">
          <p className="sr-only">Loading dashboard…</p>
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}
      {dashboardQuery.isError && !dashboardQuery.data ? (
        <Alert data-testid="dashboard-load-error" variant="error">
          {apiErrorMessage(dashboardQuery.error, "Failed to load dashboard")}
        </Alert>
      ) : null}
      {dashboardQuery.data ? (
        <DashboardView
          dashboard={dashboardQuery.data}
          isRefetching={dashboardQuery.isRefetching}
          isSyncing={syncAll.isPending}
          lastSyncError={actionError}
          lastSyncMessage={actionMessage}
          onSyncAll={handleSyncAll}
        />
      ) : null}
    </AppShellLayout>
  );
}
