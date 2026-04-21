import type { ReactElement } from "react";

import {
  Button,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "@compozy/ui";
import { Link } from "@tanstack/react-router";

import type { Run } from "@/systems/runs";

import type { WorkflowSummary } from "../types";

export interface WorkflowInventoryViewProps {
  workflows: WorkflowSummary[];
  isLoading: boolean;
  isRefetching: boolean;
  error?: string | null;
  workspaceName: string;
  onSyncAll: () => void;
  onSyncOne: (slug: string) => void;
  onStartRun: (slug: string) => void;
  onArchive: (slug: string) => void;
  isSyncingAll: boolean;
  pendingSyncSlug: string | null;
  pendingStartSlug: string | null;
  pendingArchiveSlug: string | null;
  startedRun?: Run | null;
  lastActionMessage?: string | null;
  lastActionError?: string | null;
}

export function WorkflowInventoryView(props: WorkflowInventoryViewProps): ReactElement {
  const {
    workflows,
    isLoading,
    isRefetching,
    error,
    workspaceName,
    onSyncAll,
    onSyncOne,
    onStartRun,
    onArchive,
    isSyncingAll,
    pendingSyncSlug,
    pendingStartSlug,
    pendingArchiveSlug,
    startedRun,
    lastActionMessage,
    lastActionError,
  } = props;

  const active = workflows.filter(workflow => !workflow.archived_at);
  const archived = workflows.filter(workflow => workflow.archived_at);

  return (
    <div className="space-y-6" data-testid="workflow-inventory-view">
      <SectionHeading
        actions={
          <Button
            data-testid="workflow-inventory-sync-all"
            disabled={isSyncingAll}
            onClick={onSyncAll}
            size="sm"
          >
            {isSyncingAll ? "Syncing…" : "Sync all"}
          </Button>
        }
        description={`Workflows registered with ${workspaceName}.`}
        eyebrow="Workflows"
        title="Workflow inventory"
      />

      {lastActionError ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="workflow-inventory-error"
          role="alert"
        >
          {lastActionError}
        </p>
      ) : null}
      {lastActionMessage ? (
        <p
          className="rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3 text-sm text-muted-foreground"
          data-testid="workflow-inventory-action-success"
        >
          {lastActionMessage}
        </p>
      ) : null}
      {startedRun ? (
        <p
          className="rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3 text-sm text-muted-foreground"
          data-testid="workflow-inventory-start-success"
        >
          Started run{" "}
          <Link
            className="font-mono text-accent hover:underline"
            data-testid="workflow-inventory-start-success-link"
            params={{ runId: startedRun.run_id }}
            to="/runs/$runId"
          >
            {startedRun.run_id}
          </Link>{" "}
          for {startedRun.workflow_slug ?? "the workflow"}.
        </p>
      ) : null}

      {error ? (
        <p
          className="rounded-[var(--radius-md)] border border-[color:var(--color-danger)] bg-black/20 px-4 py-3 text-sm text-[color:var(--color-danger)]"
          data-testid="workflow-inventory-load-error"
          role="alert"
        >
          {error}
        </p>
      ) : null}

      {isLoading ? (
        <p className="text-sm text-muted-foreground" data-testid="workflow-inventory-loading">
          Loading workflows…
        </p>
      ) : null}

      {!isLoading && workflows.length === 0 ? (
        <SurfaceCard data-testid="workflow-inventory-empty">
          <SurfaceCardHeader>
            <div>
              <SurfaceCardEyebrow>Empty</SurfaceCardEyebrow>
              <SurfaceCardTitle>No workflows yet</SurfaceCardTitle>
              <SurfaceCardDescription>
                Register a workflow through <code>compozy sync</code> or run the sync action above
                to let the daemon pick up workflow artifacts from this workspace.
              </SurfaceCardDescription>
            </div>
          </SurfaceCardHeader>
        </SurfaceCard>
      ) : null}

      {active.length > 0 ? (
        <div className="space-y-3" data-testid="workflow-inventory-active">
          <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
            Active · {active.length}
          </p>
          <ul className="grid gap-3">
            {active.map(workflow => (
              <WorkflowRow
                key={workflow.id}
                onArchive={() => onArchive(workflow.slug)}
                onStartRun={() => onStartRun(workflow.slug)}
                onSync={() => onSyncOne(workflow.slug)}
                pendingArchive={pendingArchiveSlug === workflow.slug}
                pendingStart={pendingStartSlug === workflow.slug}
                pendingSync={pendingSyncSlug === workflow.slug}
                workflow={workflow}
              />
            ))}
          </ul>
        </div>
      ) : null}

      {archived.length > 0 ? (
        <div className="space-y-3" data-testid="workflow-inventory-archived">
          <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
            Archived · {archived.length}
          </p>
          <ul className="grid gap-3">
            {archived.map(workflow => (
              <ArchivedRow key={workflow.id} workflow={workflow} />
            ))}
          </ul>
        </div>
      ) : null}

      {isRefetching ? (
        <p className="text-xs text-muted-foreground" data-testid="workflow-inventory-refreshing">
          refreshing…
        </p>
      ) : null}
    </div>
  );
}

function WorkflowRow({
  workflow,
  onSync,
  onStartRun,
  onArchive,
  pendingSync,
  pendingStart,
  pendingArchive,
}: {
  workflow: WorkflowSummary;
  onSync: () => void;
  onStartRun: () => void;
  onArchive: () => void;
  pendingSync: boolean;
  pendingStart: boolean;
  pendingArchive: boolean;
}): ReactElement {
  return (
    <li>
      <SurfaceCard data-testid={`workflow-row-${workflow.slug}`}>
        <SurfaceCardHeader>
          <div>
            <SurfaceCardEyebrow>Workflow</SurfaceCardEyebrow>
            <SurfaceCardTitle>
              <Link
                className="text-foreground hover:underline"
                data-testid={`workflow-open-${workflow.slug}`}
                params={{ slug: workflow.slug }}
                to="/workflows/$slug/tasks"
              >
                {workflow.slug}
              </Link>
            </SurfaceCardTitle>
            <SurfaceCardDescription>
              {workflow.last_synced_at
                ? `Last synced ${new Date(workflow.last_synced_at).toLocaleString()}`
                : "Not synced yet"}
            </SurfaceCardDescription>
          </div>
          <StatusBadge tone="info">active</StatusBadge>
        </SurfaceCardHeader>
        <SurfaceCardBody className="flex flex-wrap gap-2">
          <Link
            className="inline-flex items-center justify-center rounded-[var(--radius-sm)] border border-border bg-black/10 px-3 py-1 text-sm text-foreground hover:underline"
            data-testid={`workflow-view-board-${workflow.slug}`}
            params={{ slug: workflow.slug }}
            to="/workflows/$slug/tasks"
          >
            Open task board
          </Link>
          <Link
            className="inline-flex items-center justify-center rounded-[var(--radius-sm)] border border-border bg-black/10 px-3 py-1 text-sm text-foreground hover:underline"
            data-testid={`workflow-view-spec-${workflow.slug}`}
            params={{ slug: workflow.slug }}
            to="/workflows/$slug/spec"
          >
            Spec
          </Link>
          <Link
            className="inline-flex items-center justify-center rounded-[var(--radius-sm)] border border-border bg-black/10 px-3 py-1 text-sm text-foreground hover:underline"
            data-testid={`workflow-view-memory-${workflow.slug}`}
            params={{ slug: workflow.slug }}
            to="/memory/$slug"
          >
            Memory
          </Link>
          <Button
            data-testid={`workflow-start-${workflow.slug}`}
            disabled={pendingStart}
            onClick={onStartRun}
            size="sm"
          >
            {pendingStart ? "Starting…" : "Start run"}
          </Button>
          <Button
            data-testid={`workflow-sync-${workflow.slug}`}
            disabled={pendingSync}
            onClick={onSync}
            size="sm"
            variant="secondary"
          >
            {pendingSync ? "Syncing…" : "Sync"}
          </Button>
          <Button
            data-testid={`workflow-archive-${workflow.slug}`}
            disabled={pendingArchive}
            onClick={onArchive}
            size="sm"
            variant="ghost"
          >
            {pendingArchive ? "Archiving…" : "Archive"}
          </Button>
        </SurfaceCardBody>
      </SurfaceCard>
    </li>
  );
}

function ArchivedRow({ workflow }: { workflow: WorkflowSummary }): ReactElement {
  return (
    <li>
      <SurfaceCard data-testid={`workflow-archived-${workflow.slug}`}>
        <SurfaceCardHeader>
          <div>
            <SurfaceCardEyebrow>Archived</SurfaceCardEyebrow>
            <SurfaceCardTitle>{workflow.slug}</SurfaceCardTitle>
            <SurfaceCardDescription>
              {workflow.archived_at
                ? `Archived ${new Date(workflow.archived_at).toLocaleString()}`
                : "Archived"}
            </SurfaceCardDescription>
          </div>
          <StatusBadge tone="warning">archived</StatusBadge>
        </SurfaceCardHeader>
      </SurfaceCard>
    </li>
  );
}
