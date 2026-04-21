import { useState, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { DashboardView, useDashboard } from "@/systems/dashboard";
import { useSyncWorkflows } from "@/systems/workflows";

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
      setActionMessage(
        `Sync completed — ${result.workflows_scanned ?? 0} workflow${(result.workflows_scanned ?? 0) === 1 ? "" : "s"} scanned.`
      );
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
        <p className="text-sm text-muted-foreground" data-testid="dashboard-loading">
          Loading dashboard…
        </p>
      ) : null}
      {dashboardQuery.isError && !dashboardQuery.data ? (
        <p
          className="text-sm text-[color:var(--color-danger)]"
          data-testid="dashboard-load-error"
          role="alert"
        >
          {apiErrorMessage(dashboardQuery.error, "Failed to load dashboard")}
        </p>
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
