import { useState, type ReactElement } from "react";

import { createFileRoute } from "@tanstack/react-router";

import { apiErrorMessage, toTransportError } from "@/lib/api-client";
import { AppShellLayout, useActiveWorkspaceContext } from "@/systems/app-shell";
import { useStartWorkflowRun, type Run } from "@/systems/runs";
import {
  type ArchiveConfirmationState,
  formatWorkflowSyncResult,
  useArchiveWorkflow,
  useSyncWorkflows,
  useWorkflows,
  WorkflowInventoryView,
} from "@/systems/workflows";

export const Route = createFileRoute("/_app/workflows")({
  component: WorkflowsRoute,
});

function toInt(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function archiveConfirmationFromError(
  error: unknown,
  slug: string
): ArchiveConfirmationState | null {
  const transport = toTransportError(error);
  if (transport?.code !== "workflow_force_required") {
    return null;
  }

  const details = transport.details ?? {};
  const workflowSlug =
    typeof details.workflow_slug === "string" && details.workflow_slug.trim().length > 0
      ? details.workflow_slug.trim()
      : slug;

  return {
    slug: workflowSlug,
    archiveReason:
      typeof details.archive_reason === "string" ? details.archive_reason : "pending local work",
    taskNonTerminal: Math.max(toInt(details.task_non_terminal), toInt(details.task_pending)),
    reviewUnresolved: toInt(details.review_unresolved),
    reviewTotal: toInt(details.review_total),
  };
}

function formatArchiveMessage(
  slug: string,
  result: {
    archived?: boolean;
    forced?: boolean;
    completed_tasks?: number;
    resolved_review_issues?: number;
  }
): string {
  if (!result.archived) {
    return `${slug} is already archived (no-op).`;
  }
  if (result.forced) {
    const completedTasks = result.completed_tasks ?? 0;
    const resolvedReviewIssues = result.resolved_review_issues ?? 0;
    return `Archived ${slug} after completing ${completedTasks} task${completedTasks === 1 ? "" : "s"} and resolving ${resolvedReviewIssues} review issue${resolvedReviewIssues === 1 ? "" : "s"}.`;
  }
  return `Archived ${slug}.`;
}

function WorkflowsRoute(): ReactElement {
  const { activeWorkspace, workspaces, onSwitchWorkspace } = useActiveWorkspaceContext();
  const workflowsQuery = useWorkflows(activeWorkspace.id);
  const sync = useSyncWorkflows();
  const startRun = useStartWorkflowRun();
  const archive = useArchiveWorkflow();

  const [pendingSyncSlug, setPendingSyncSlug] = useState<string | null>(null);
  const [pendingStartSlug, setPendingStartSlug] = useState<string | null>(null);
  const [pendingArchiveSlug, setPendingArchiveSlug] = useState<string | null>(null);
  const [startedRun, setStartedRun] = useState<Run | null>(null);
  const [actionMessage, setActionMessage] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [archiveConfirmation, setArchiveConfirmation] = useState<ArchiveConfirmationState | null>(
    null
  );

  async function handleSyncAll() {
    setActionMessage(null);
    setActionError(null);
    setStartedRun(null);
    setArchiveConfirmation(null);
    try {
      const result = await sync.mutateAsync({ workspaceId: activeWorkspace.id });
      setActionMessage(formatWorkflowSyncResult(result));
    } catch (error) {
      setActionError(apiErrorMessage(error, "Sync failed"));
    }
  }

  async function handleSyncOne(slug: string) {
    setActionMessage(null);
    setActionError(null);
    setStartedRun(null);
    setArchiveConfirmation(null);
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

  async function handleStartRun(slug: string) {
    setActionMessage(null);
    setActionError(null);
    setStartedRun(null);
    setArchiveConfirmation(null);
    setPendingStartSlug(slug);
    try {
      const run = await startRun.mutateAsync({
        workspaceId: activeWorkspace.id,
        slug,
        body: { presentation_mode: "detach" },
      });
      setStartedRun(run);
    } catch (error) {
      setActionError(apiErrorMessage(error, `Failed to start run for ${slug}`));
    } finally {
      setPendingStartSlug(null);
    }
  }

  async function handleArchive(slug: string) {
    setActionMessage(null);
    setActionError(null);
    setStartedRun(null);
    setArchiveConfirmation(null);
    setPendingArchiveSlug(slug);
    try {
      const result = await archive.mutateAsync({
        workspaceId: activeWorkspace.id,
        slug,
      });
      setActionMessage(formatArchiveMessage(slug, result));
    } catch (error) {
      const confirmation =
        archiveConfirmationFromError(error, slug) ??
        archiveConfirmationFromError(archive.error, slug);
      if (confirmation) {
        setArchiveConfirmation(confirmation);
        return;
      }
      setActionError(apiErrorMessage(error, `Failed to archive ${slug}`));
    } finally {
      setPendingArchiveSlug(null);
    }
  }

  async function handleConfirmArchive(slug: string) {
    setActionMessage(null);
    setActionError(null);
    setStartedRun(null);
    setPendingArchiveSlug(slug);
    try {
      const result = await archive.mutateAsync({
        workspaceId: activeWorkspace.id,
        slug,
        force: true,
      });
      setArchiveConfirmation(null);
      setActionMessage(formatArchiveMessage(slug, result));
    } catch (error) {
      setArchiveConfirmation(null);
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
        isReadOnly={activeWorkspace.read_only}
        isSyncingAll={sync.isPending && pendingSyncSlug === null}
        archiveConfirmation={archiveConfirmation}
        lastActionError={actionError}
        lastActionMessage={actionMessage}
        onArchive={handleArchive}
        onCancelArchiveConfirmation={() => setArchiveConfirmation(null)}
        onConfirmArchiveConfirmation={handleConfirmArchive}
        onStartRun={handleStartRun}
        onSyncAll={handleSyncAll}
        onSyncOne={handleSyncOne}
        pendingArchiveSlug={pendingArchiveSlug}
        pendingStartSlug={pendingStartSlug}
        pendingSyncSlug={pendingSyncSlug}
        startedRun={startedRun}
        workflows={workflowsQuery.data ?? []}
        workspaceName={activeWorkspace.name}
      />
    </AppShellLayout>
  );
}
