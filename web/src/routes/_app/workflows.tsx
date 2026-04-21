import { useState, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";

import { apiErrorMessage } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import {
  useArchiveWorkflow,
  useSyncWorkflows,
  useWorkflows,
  WorkflowInventoryView,
} from "@/systems/workflows";

export const Route = createFileRoute("/_app/workflows")({
  component: WorkflowsRoute,
});

function WorkflowsRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const workflowsQuery = useWorkflows(activeWorkspace.id);
  const sync = useSyncWorkflows();
  const archive = useArchiveWorkflow();

  const [pendingSyncSlug, setPendingSyncSlug] = useState<string | null>(null);
  const [pendingArchiveSlug, setPendingArchiveSlug] = useState<string | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  async function handleSyncAll() {
    setActionMessage(null);
    setActionError(null);
    try {
      const result = await sync.mutateAsync({ workspaceId: activeWorkspace.id });
      setActionMessage(
        `Sync completed — ${result.workflows_scanned ?? 0} workflow${(result.workflows_scanned ?? 0) === 1 ? "" : "s"} scanned.`
      );
    } catch (error) {
      setActionError(apiErrorMessage(error, "Sync failed"));
    }
  }

  async function handleSyncOne(slug: string) {
    setActionMessage(null);
    setActionError(null);
    setPendingSyncSlug(slug);
    try {
      const result = await sync.mutateAsync({
        workspaceId: activeWorkspace.id,
        workflowSlug: slug,
      });
      setActionMessage(
        `Synced ${slug} — ${result.task_items_upserted ?? 0} task${(result.task_items_upserted ?? 0) === 1 ? "" : "s"} upserted.`
      );
    } catch (error) {
      setActionError(apiErrorMessage(error, `Failed to sync ${slug}`));
    } finally {
      setPendingSyncSlug(null);
    }
  }

  async function handleArchive(slug: string) {
    setActionMessage(null);
    setActionError(null);
    setPendingArchiveSlug(slug);
    try {
      const result = await archive.mutateAsync({
        workspaceId: activeWorkspace.id,
        slug,
      });
      setActionMessage(
        result.archived ? `Archived ${slug}.` : `${slug} is already archived (no-op).`
      );
    } catch (error) {
      setActionError(apiErrorMessage(error, `Failed to archive ${slug}`));
    } finally {
      setPendingArchiveSlug(null);
    }
  }

  return (
    <AppShellLayout
      activeWorkspace={activeWorkspace}
      onSwitchWorkspace={onSwitchWorkspace}
      workspaces={workspaces}
    >
      <WorkflowInventoryView
        error={
          workflowsQuery.isError
            ? apiErrorMessage(workflowsQuery.error, "Failed to load workflows")
            : null
        }
        isLoading={workflowsQuery.isLoading}
        isRefetching={workflowsQuery.isRefetching}
        isSyncingAll={sync.isPending && pendingSyncSlug === null}
        lastActionError={actionError}
        lastActionMessage={actionMessage}
        onArchive={handleArchive}
        onSyncAll={handleSyncAll}
        onSyncOne={handleSyncOne}
        pendingArchiveSlug={pendingArchiveSlug}
        pendingSyncSlug={pendingSyncSlug}
        workflows={workflowsQuery.data ?? []}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
