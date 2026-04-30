import type { ReactElement } from "react";

import { Archive, BookOpen, FileText, Play, RefreshCw } from "lucide-react";

import {
  Alert,
  Button,
  EmptyState,
  SectionHeading,
  SkeletonRow,
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

function isWorkflowCompleted(workflow: WorkflowSummary): boolean {
  if (workflow.archived_at) return false;
  return workflow.archive_eligible === true;
}

export interface WorkflowInventoryViewProps {
  workflows: WorkflowSummary[];
  isLoading: boolean;
  isRefetching: boolean;
  error?: string | null;
  workspaceName: string;
  isReadOnly?: boolean;
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
    isReadOnly = false,
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

  const archived = workflows.filter(workflow => Boolean(workflow.archived_at));
  const completed = workflows.filter(isWorkflowCompleted);
  const active = workflows.filter(
    workflow => !workflow.archived_at && !isWorkflowCompleted(workflow)
  );

  return (
    <div className="space-y-6" data-testid="workflow-inventory-view">
      <SectionHeading
        actions={
          <Button
            data-testid="workflow-inventory-sync-all"
            disabled={isSyncingAll || isReadOnly}
            icon={<RefreshCw className="size-4" />}
            loading={isSyncingAll}
            onClick={onSyncAll}
            size="sm"
          >
            Sync all
          </Button>
        }
        description={`Workflows registered with ${workspaceName}.`}
        eyebrow="Workflows"
        title="Workflow inventory"
      />

      {lastActionError ? (
        <Alert data-testid="workflow-inventory-error" variant="error">
          {lastActionError}
        </Alert>
      ) : null}
      {isReadOnly ? (
        <Alert data-testid="workflow-inventory-readonly" variant="warning">
          Filesystem actions are read-only for this workspace.
        </Alert>
      ) : null}
      {lastActionMessage ? (
        <Alert data-testid="workflow-inventory-action-success" variant="success">
          {lastActionMessage}
        </Alert>
      ) : null}
      {startedRun ? (
        <Alert data-testid="workflow-inventory-start-success" variant="success">
          Started run{" "}
          <Link
            className="font-mono text-primary hover:underline"
            data-testid="workflow-inventory-start-success-link"
            params={{ runId: startedRun.run_id }}
            to="/runs/$runId"
          >
            {startedRun.run_id}
          </Link>{" "}
          for {startedRun.workflow_slug ?? "the workflow"}.
        </Alert>
      ) : null}

      {error ? (
        <Alert data-testid="workflow-inventory-load-error" variant="error">
          {error}
        </Alert>
      ) : null}

      {isLoading ? (
        <div className="space-y-2" data-testid="workflow-inventory-loading">
          <SkeletonRow />
          <SkeletonRow />
          <SkeletonRow />
        </div>
      ) : null}

      {!isLoading && workflows.length === 0 ? (
        <EmptyState
          action={
            <Button
              disabled={isSyncingAll || isReadOnly}
              icon={<RefreshCw className="size-4" />}
              loading={isSyncingAll}
              onClick={onSyncAll}
              size="sm"
            >
              Sync all
            </Button>
          }
          data-testid="workflow-inventory-empty"
          description={
            <>
              Register a workflow through <code>compozy sync</code> or run sync here to let the
              daemon pick up workflow artifacts from this workspace.
            </>
          }
          icon={<FileText className="size-4" aria-hidden />}
          title="No workflows yet"
        />
      ) : null}

      {active.length > 0 ? (
        <div className="space-y-3" data-testid="workflow-inventory-active">
          <p className="eyebrow text-muted-foreground">Active · {active.length}</p>
          <ul className="grid gap-3">
            {active.map(workflow => (
              <WorkflowRow
                key={workflow.id}
                onArchive={() => onArchive(workflow.slug)}
                onStartRun={() => onStartRun(workflow.slug)}
                onSync={() => onSyncOne(workflow.slug)}
                readOnly={isReadOnly}
                pendingArchive={pendingArchiveSlug === workflow.slug}
                pendingStart={pendingStartSlug === workflow.slug}
                pendingSync={pendingSyncSlug === workflow.slug}
                workflow={workflow}
              />
            ))}
          </ul>
        </div>
      ) : null}

      {completed.length > 0 ? (
        <div className="space-y-3" data-testid="workflow-inventory-completed">
          <p className="eyebrow text-muted-foreground">Completed · {completed.length}</p>
          <ul className="grid gap-3">
            {completed.map(workflow => (
              <WorkflowRow
                key={workflow.id}
                onArchive={() => onArchive(workflow.slug)}
                onStartRun={() => onStartRun(workflow.slug)}
                onSync={() => onSyncOne(workflow.slug)}
                readOnly={isReadOnly}
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
          <p className="eyebrow text-muted-foreground">Archived · {archived.length}</p>
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
  readOnly,
}: {
  workflow: WorkflowSummary;
  onSync: () => void;
  onStartRun: () => void;
  onArchive: () => void;
  pendingSync: boolean;
  pendingStart: boolean;
  pendingArchive: boolean;
  readOnly: boolean;
}): ReactElement {
  const isCompleted = isWorkflowCompleted(workflow);
  const canStartRun = !isCompleted && workflow.can_start_run !== false;
  const startBlockReason = workflow.start_block_reason?.trim() ?? "";
  const startBlockLabel = isCompleted ? "completed" : startBlockReason;
  return (
    <li>
      <SurfaceCard data-interactive="true" data-testid={`workflow-row-${workflow.slug}`}>
        <SurfaceCardHeader>
          <div className="min-w-0">
            <SurfaceCardEyebrow>Workflow</SurfaceCardEyebrow>
            <SurfaceCardTitle>
              <Link
                className="block truncate text-foreground hover:underline"
                data-testid={`workflow-open-${workflow.slug}`}
                params={{ slug: workflow.slug }}
                to="/workflows/$slug/tasks"
                title={workflow.slug}
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
          {isCompleted ? (
            <StatusBadge tone="success">completed</StatusBadge>
          ) : (
            <StatusBadge tone="info">active</StatusBadge>
          )}
        </SurfaceCardHeader>
        <SurfaceCardBody className="flex flex-wrap gap-2">
          <Link
            className="inline-flex items-center justify-center gap-2 rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 py-1.5 text-sm text-foreground transition-colors hover:border-border-strong hover:bg-surface-hover"
            data-testid={`workflow-view-board-${workflow.slug}`}
            params={{ slug: workflow.slug }}
            to="/workflows/$slug/tasks"
          >
            <BookOpen className="size-3.5" aria-hidden />
            Open task board
          </Link>
          <Link
            className="inline-flex items-center justify-center gap-2 rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 py-1.5 text-sm text-foreground transition-colors hover:border-border-strong hover:bg-surface-hover"
            data-testid={`workflow-view-spec-${workflow.slug}`}
            params={{ slug: workflow.slug }}
            to="/workflows/$slug/spec"
          >
            <FileText className="size-3.5" aria-hidden />
            Spec
          </Link>
          <Link
            className="inline-flex items-center justify-center gap-2 rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 py-1.5 text-sm text-foreground transition-colors hover:border-border-strong hover:bg-surface-hover"
            data-testid={`workflow-view-memory-${workflow.slug}`}
            params={{ slug: workflow.slug }}
            to="/memory/$slug"
          >
            <BookOpen className="size-3.5" aria-hidden />
            Memory
          </Link>
          {canStartRun ? (
            <Button
              data-testid={`workflow-start-${workflow.slug}`}
              disabled={pendingStart || readOnly}
              icon={<Play className="size-4" />}
              loading={pendingStart}
              onClick={onStartRun}
              size="sm"
            >
              Start run
            </Button>
          ) : isCompleted ? null : (
            <StatusBadge data-testid={`workflow-start-blocked-${workflow.slug}`} tone="warning">
              {startBlockLabel || "not startable"}
            </StatusBadge>
          )}
          <Button
            data-testid={`workflow-sync-${workflow.slug}`}
            disabled={pendingSync || readOnly}
            icon={<RefreshCw className="size-4" />}
            loading={pendingSync}
            onClick={onSync}
            size="sm"
            variant="secondary"
          >
            Sync
          </Button>
          <Button
            data-testid={`workflow-archive-${workflow.slug}`}
            disabled={pendingArchive || readOnly}
            icon={<Archive className="size-4" />}
            loading={pendingArchive}
            onClick={onArchive}
            size="sm"
            variant="ghost"
          >
            Archive
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
          <StatusBadge tone="neutral">archived</StatusBadge>
        </SurfaceCardHeader>
      </SurfaceCard>
    </li>
  );
}
