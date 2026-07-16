import { useId, useState, type ReactElement } from "react";

import { AlertTriangle, Archive, BookOpen, FileText, Play, RefreshCw } from "lucide-react";

import {
  Alert,
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
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

import type { WorkflowSummary, WorkPackageSummary } from "../types";

function isWorkflowCompleted(workflow: WorkflowSummary): boolean {
  if (workflow.archived_at) return false;
  return workflow.archive_eligible === true;
}

export interface ArchiveConfirmationState {
  slug: string;
  archiveReason: string;
  taskNonTerminal: number;
  reviewUnresolved: number;
  reviewTotal: number;
}

export interface WorkflowRunRequest {
  packageId?: string;
  allowOutOfOrder?: boolean;
}

function pluralize(count: number, singular: string): string {
  return `${count} ${singular}${count === 1 ? "" : "s"}`;
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
  onStartRun: (slug: string, request?: WorkflowRunRequest) => void | Promise<void>;
  onArchive: (slug: string) => void;
  onConfirmArchiveConfirmation: (slug: string) => void;
  onCancelArchiveConfirmation: () => void;
  isSyncingAll: boolean;
  pendingSyncSlug: string | null;
  pendingStartSlug: string | null;
  pendingArchiveSlug: string | null;
  archiveConfirmation?: ArchiveConfirmationState | null;
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
    onConfirmArchiveConfirmation,
    onCancelArchiveConfirmation,
    isSyncingAll,
    pendingSyncSlug,
    pendingStartSlug,
    pendingArchiveSlug,
    archiveConfirmation = null,
    startedRun,
    lastActionMessage,
    lastActionError,
  } = props;

  const archiveConfirmationPending =
    archiveConfirmation !== null && pendingArchiveSlug === archiveConfirmation.slug;
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

      <AlertDialog open={Boolean(archiveConfirmation)}>
        {archiveConfirmation ? (
          <AlertDialogContent data-testid="workflow-archive-confirmation">
            <AlertDialogHeader>
              <AlertDialogTitle>Archive {archiveConfirmation.slug}?</AlertDialogTitle>
              <AlertDialogDescription>
                This workflow still has pending local work. If you continue, Compozy will complete
                pending tasks, resolve local review issues, sync the workflow, and then archive it.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <div className="space-y-3 px-6 pb-6">
              <Alert
                data-testid="workflow-archive-confirmation-warning"
                icon={<AlertTriangle className="size-4" />}
                title="Pending local work"
                variant="warning"
              >
                {archiveConfirmation.archiveReason}.
              </Alert>
              <div className="rounded-[var(--radius-lg)] border border-border-subtle bg-[color:var(--surface-inset)] px-4 py-3 text-sm text-muted-foreground">
                {archiveConfirmation.taskNonTerminal > 0 ? (
                  <p data-testid="workflow-archive-confirmation-tasks">
                    {pluralize(archiveConfirmation.taskNonTerminal, "task")} will be marked as
                    completed.
                  </p>
                ) : null}
                {archiveConfirmation.reviewUnresolved > 0 ? (
                  <p data-testid="workflow-archive-confirmation-reviews">
                    {pluralize(archiveConfirmation.reviewUnresolved, "review issue")} will be
                    resolved locally
                    {archiveConfirmation.reviewTotal > 0
                      ? ` out of ${pluralize(archiveConfirmation.reviewTotal, "issue")}`
                      : ""}
                    .
                  </p>
                ) : null}
              </div>
            </div>
            <AlertDialogFooter>
              <Button
                data-testid="workflow-archive-confirmation-cancel"
                disabled={archiveConfirmationPending}
                onClick={onCancelArchiveConfirmation}
                variant="secondary"
              >
                Cancel
              </Button>
              <Button
                className="border-[color:var(--tone-danger-border)] bg-[color:var(--tone-danger-bg)] text-[color:var(--tone-danger-text)] hover:brightness-105"
                data-testid="workflow-archive-confirmation-confirm"
                loading={archiveConfirmationPending}
                onClick={() => onConfirmArchiveConfirmation(archiveConfirmation.slug)}
              >
                Archive anyway
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        ) : null}
      </AlertDialog>

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
                onStartPackage={request => onStartRun(workflow.slug, request)}
                onSync={() => onSyncOne(workflow.slug)}
                readOnly={isReadOnly}
                pendingArchive={pendingArchiveSlug === workflow.slug}
                pendingStart={pendingStartSlug === workflow.slug}
                pendingStartReference={pendingStartSlug}
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
                onStartPackage={request => onStartRun(workflow.slug, request)}
                onSync={() => onSyncOne(workflow.slug)}
                readOnly={isReadOnly}
                pendingArchive={pendingArchiveSlug === workflow.slug}
                pendingStart={pendingStartSlug === workflow.slug}
                pendingStartReference={pendingStartSlug}
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
  onStartPackage,
  onArchive,
  pendingSync,
  pendingStart,
  pendingStartReference,
  pendingArchive,
  readOnly,
}: {
  workflow: WorkflowSummary;
  onSync: () => void;
  onStartRun: () => void;
  onStartPackage: (request: WorkflowRunRequest) => void | Promise<void>;
  onArchive: () => void;
  pendingSync: boolean;
  pendingStart: boolean;
  pendingStartReference: string | null;
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
      {(workflow.work_packages?.length ?? 0) > 0 ? (
        <WorkPackageList
          initiativeSlug={workflow.slug}
          onStartPackage={onStartPackage}
          packages={workflow.work_packages ?? []}
          pendingStartReference={pendingStartReference}
          readOnly={readOnly}
        />
      ) : null}
    </li>
  );
}

function WorkPackageList({
  initiativeSlug,
  onStartPackage,
  packages,
  pendingStartReference,
  readOnly,
}: {
  initiativeSlug: string;
  onStartPackage: (request: WorkflowRunRequest) => void | Promise<void>;
  packages: WorkPackageSummary[];
  pendingStartReference: string | null;
  readOnly: boolean;
}): ReactElement {
  const inputId = useId();
  const [query, setQuery] = useState("");
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visiblePackages = normalizedQuery
    ? packages.filter(pkg =>
        [pkg.package_id, pkg.title, pkg.outcome, pkg.reference].some(value =>
          value.toLocaleLowerCase().includes(normalizedQuery)
        )
      )
    : packages;

  return (
    <section
      aria-label={`Work Packages for ${initiativeSlug}`}
      className="ml-3 border-l border-border pl-4 pt-3 sm:ml-6 sm:pl-6"
      data-testid={`workflow-packages-${initiativeSlug}`}
    >
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="eyebrow text-muted-foreground">Work Packages · {packages.length}</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Child execution scopes for this initiative.
          </p>
        </div>
        <label className="grid gap-1 text-xs text-muted-foreground" htmlFor={inputId}>
          Filter packages
          <input
            className="h-9 w-full rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 text-sm text-foreground outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-[color:var(--color-primary)] focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] sm:w-64"
            data-testid={`workflow-packages-filter-${initiativeSlug}`}
            id={inputId}
            onChange={event => setQuery(event.currentTarget.value)}
            placeholder="ID, title, outcome"
            type="search"
            value={query}
          />
        </label>
      </div>
      {visiblePackages.length > 0 ? (
        <ul className="grid gap-2" data-testid={`workflow-packages-list-${initiativeSlug}`}>
          {visiblePackages.map(pkg => (
            <WorkPackageRow
              initiativeSlug={initiativeSlug}
              key={pkg.workflow_id}
              onStart={request => onStartPackage(request)}
              pendingStart={pendingStartReference === pkg.reference}
              pkg={pkg}
              readOnly={readOnly}
            />
          ))}
        </ul>
      ) : (
        <p
          className="rounded-[var(--radius-md)] border border-dashed border-border px-3 py-4 text-sm text-muted-foreground"
          data-testid={`workflow-packages-filter-empty-${initiativeSlug}`}
        >
          No Work Packages match “{query}”.
        </p>
      )}
    </section>
  );
}

function WorkPackageRow({
  initiativeSlug,
  onStart,
  pendingStart,
  pkg,
  readOnly,
}: {
  initiativeSlug: string;
  onStart: (request: WorkflowRunRequest) => void | Promise<void>;
  pendingStart: boolean;
  pkg: WorkPackageSummary;
  readOnly: boolean;
}): ReactElement {
  const unmetCount = pkg.unmet_dependency_count ?? 0;
  const unmetDependencies = pkg.unmet_dependencies ?? [];
  const unmetDependencyPaths = pkg.unmet_dependency_paths ?? [];
  const requiresStartConfirmation = pkg.requires_start_confirmation === true || unmetCount > 0;
  const [dependencyConfirmationOpen, setDependencyConfirmationOpen] = useState(false);
  const [dependencyConfirmationPending, setDependencyConfirmationPending] = useState(false);
  const completionText = pkg.lifecycle_complete
    ? "Compozy lifecycle complete; Git integration is not tracked"
    : "Compozy lifecycle incomplete";
  const selectionLabel = `${pkg.package_id}, ${pkg.title}. ${completionText}. ${unmetCount} unmet ${unmetCount === 1 ? "dependency" : "dependencies"}.`;
  const canStart =
    !pkg.lifecycle_complete &&
    pkg.can_start_run !== false &&
    (!requiresStartConfirmation || unmetDependencies.length > 0 || unmetDependencyPaths.length > 0);
  const startBlockReason =
    requiresStartConfirmation && unmetDependencies.length === 0 && unmetDependencyPaths.length === 0
      ? "dependency details unavailable"
      : pkg.start_block_reason || "not startable";
  const taskCounts = pkg.task_counts;

  async function handleConfirmDependencyOverride() {
    if (dependencyConfirmationPending) return;
    setDependencyConfirmationPending(true);
    try {
      await onStart({ packageId: pkg.package_id, allowOutOfOrder: true });
      setDependencyConfirmationOpen(false);
    } finally {
      setDependencyConfirmationPending(false);
    }
  }

  return (
    <li>
      <SurfaceCard data-testid={`workflow-package-${initiativeSlug}-${pkg.package_id}`}>
        <SurfaceCardHeader>
          <div className="min-w-0">
            <SurfaceCardEyebrow>{pkg.package_id}</SurfaceCardEyebrow>
            <SurfaceCardTitle>
              <Link
                aria-label={selectionLabel}
                className="block text-foreground hover:underline"
                data-testid={`workflow-package-open-${initiativeSlug}-${pkg.package_id}`}
                params={{ slug: initiativeSlug }}
                search={{ package_id: pkg.package_id }}
                to="/workflows/$slug/tasks"
              >
                {pkg.title}
              </Link>
            </SurfaceCardTitle>
            <SurfaceCardDescription>{pkg.outcome}</SurfaceCardDescription>
          </div>
          <StatusBadge
            tone={pkg.lifecycle_complete ? "success" : unmetCount > 0 ? "warning" : "info"}
          >
            {pkg.lifecycle_complete
              ? "lifecycle complete"
              : unmetCount > 0
                ? "dependencies unmet"
                : "ready"}
          </StatusBadge>
        </SurfaceCardHeader>
        <SurfaceCardBody className="space-y-3">
          <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2 xl:grid-cols-4">
            <p data-testid={`workflow-package-lifecycle-${pkg.package_id}`}>{completionText}.</p>
            <p data-testid={`workflow-package-readiness-${pkg.package_id}`}>
              {unmetCount > 0
                ? `${pluralize(unmetCount, "unmet dependency")}.`
                : "All declared dependencies are complete."}
            </p>
            <p>
              {pkg.independently_eligible
                ? "May be developed independently of an eligible peer."
                : "Follows the declared dependency order."}
            </p>
            <p>
              {taskCounts
                ? `${taskCounts.completed}/${taskCounts.total} tasks complete`
                : "Task counts unavailable"}
              {` · ${pluralize(pkg.unresolved_reviews ?? 0, "unresolved review")}`}
              {` · ${pluralize(pkg.active_runs ?? 0, "active run")}`}
            </p>
          </div>
          {(pkg.dependencies?.length ?? 0) > 0 ? (
            <ul
              aria-label={`Dependencies for ${pkg.package_id}`}
              className="flex flex-wrap gap-2"
              data-testid={`workflow-package-dependencies-${pkg.package_id}`}
            >
              {pkg.dependencies?.map(dependency => (
                <li
                  className="rounded-[var(--radius-sm)] border border-border-subtle bg-[color:var(--surface-inset)] px-2 py-1 text-xs text-muted-foreground"
                  key={`${dependency.package_id}-${dependency.rationale}`}
                >
                  <span className="font-mono text-foreground">{dependency.package_id}</span>
                  {` — ${dependency.rationale}`}
                </li>
              ))}
            </ul>
          ) : null}
          <div className="flex flex-wrap gap-2">
            <PackageLink
              label="Task board"
              packageId={pkg.package_id}
              slug={initiativeSlug}
              testId={`workflow-package-tasks-${initiativeSlug}-${pkg.package_id}`}
              to="/workflows/$slug/tasks"
            />
            <PackageLink
              label="Spec + plan"
              packageId={pkg.package_id}
              slug={initiativeSlug}
              testId={`workflow-package-spec-${initiativeSlug}-${pkg.package_id}`}
              to="/workflows/$slug/spec"
            />
            <PackageLink
              label="Memory"
              packageId={pkg.package_id}
              slug={initiativeSlug}
              testId={`workflow-package-memory-${initiativeSlug}-${pkg.package_id}`}
              to="/memory/$slug"
            />
            {canStart ? (
              <Button
                data-testid={`workflow-package-start-${initiativeSlug}-${pkg.package_id}`}
                disabled={pendingStart || readOnly}
                icon={<Play className="size-4" />}
                loading={pendingStart}
                onClick={() => {
                  if (requiresStartConfirmation) {
                    setDependencyConfirmationOpen(true);
                    return;
                  }
                  void onStart({ packageId: pkg.package_id });
                }}
                size="sm"
              >
                Start package run
              </Button>
            ) : pkg.lifecycle_complete ? null : (
              <StatusBadge tone="warning">{startBlockReason}</StatusBadge>
            )}
          </div>
        </SurfaceCardBody>
      </SurfaceCard>
      <AlertDialog open={dependencyConfirmationOpen}>
        <AlertDialogContent
          data-testid={`workflow-package-dependency-confirmation-${initiativeSlug}-${pkg.package_id}`}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>Start {pkg.reference} out of order?</AlertDialogTitle>
            <AlertDialogDescription>
              This run has unmet dependencies. Continuing authorizes only this package run and does
              not change the work package plan.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-3 px-6 pb-6">
            <Alert
              icon={<AlertTriangle className="size-4" />}
              title="Unmet dependency details"
              variant="warning"
            >
              Review each prerequisite before authorizing this one out-of-order run.
            </Alert>
            <ul
              aria-label={`Unmet dependency details for ${pkg.package_id}`}
              className="space-y-2 rounded-[var(--radius-lg)] border border-border-subtle bg-[color:var(--surface-inset)] px-4 py-3 text-sm text-muted-foreground"
              data-testid={`workflow-package-dependency-confirmation-dependencies-${initiativeSlug}-${pkg.package_id}`}
            >
              {unmetDependencies.map(dependency => (
                <li key={`${dependency.package_id}-${dependency.rationale}`}>
                  <span className="font-mono text-foreground">{dependency.package_id}</span>
                  {` — ${dependency.title}: ${dependency.rationale}`}
                </li>
              ))}
              {unmetDependencyPaths.map(path => (
                <li key={path.package_ids.join("/")}>
                  <p>Transitive path: {path.package_ids.join(" → ")}</p>
                  <ul className="mt-1 space-y-1">
                    {path.dependencies.map(dependency => (
                      <li key={`${dependency.package_id}-${dependency.rationale}`}>
                        <span className="font-mono text-foreground">{dependency.package_id}</span>
                        {` — ${dependency.title}: ${dependency.rationale}`}
                      </li>
                    ))}
                  </ul>
                </li>
              ))}
            </ul>
          </div>
          <AlertDialogFooter>
            <Button
              data-testid={`workflow-package-dependency-confirmation-cancel-${initiativeSlug}-${pkg.package_id}`}
              disabled={dependencyConfirmationPending}
              onClick={() => setDependencyConfirmationOpen(false)}
              variant="secondary"
            >
              Cancel
            </Button>
            <Button
              data-testid={`workflow-package-dependency-confirmation-confirm-${initiativeSlug}-${pkg.package_id}`}
              loading={dependencyConfirmationPending}
              onClick={() => void handleConfirmDependencyOverride()}
            >
              Start out of order
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </li>
  );
}

function PackageLink({
  label,
  packageId,
  slug,
  testId,
  to,
}: {
  label: string;
  packageId: string;
  slug: string;
  testId: string;
  to: "/memory/$slug" | "/workflows/$slug/spec" | "/workflows/$slug/tasks";
}): ReactElement {
  return (
    <Link
      className="inline-flex items-center justify-center rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 py-1.5 text-sm text-foreground transition-colors hover:border-border-strong hover:bg-surface-hover"
      data-testid={testId}
      params={{ slug }}
      search={{ package_id: packageId }}
      to={to}
    >
      {label}
    </Link>
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
      {(workflow.work_packages?.length ?? 0) > 0 ? (
        <ul
          aria-label={`Archived Work Packages for ${workflow.slug}`}
          className="ml-3 grid gap-2 border-l border-border pl-4 pt-3 sm:ml-6 sm:pl-6"
        >
          {workflow.work_packages?.map(pkg => (
            <li
              className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2"
              key={pkg.workflow_id}
            >
              <p className="font-mono text-xs text-muted-foreground">{pkg.package_id}</p>
              <p className="mt-1 text-sm font-medium text-foreground">{pkg.title}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {pkg.lifecycle_complete
                  ? "Compozy lifecycle complete; Git integration is not tracked."
                  : "Compozy lifecycle incomplete."}
              </p>
            </li>
          ))}
        </ul>
      ) : null}
    </li>
  );
}
