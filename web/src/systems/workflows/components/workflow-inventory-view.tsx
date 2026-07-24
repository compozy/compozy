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

import type { WorkflowSummary, TaskGroupSummary } from "../types";

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
  taskGroupId?: string;
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
                onStartTaskGroup={request => onStartRun(workflow.slug, request)}
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
                onStartTaskGroup={request => onStartRun(workflow.slug, request)}
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
  onStartTaskGroup,
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
  onStartTaskGroup: (request: WorkflowRunRequest) => void | Promise<void>;
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
      {(workflow.task_groups?.length ?? 0) > 0 ? (
        <TaskGroupList
          initiativeSlug={workflow.slug}
          onStartTaskGroup={onStartTaskGroup}
          taskGroups={workflow.task_groups ?? []}
          pendingStartReference={pendingStartReference}
          readOnly={readOnly}
        />
      ) : null}
    </li>
  );
}

function TaskGroupList({
  initiativeSlug,
  onStartTaskGroup,
  taskGroups,
  pendingStartReference,
  readOnly,
}: {
  initiativeSlug: string;
  onStartTaskGroup: (request: WorkflowRunRequest) => void | Promise<void>;
  taskGroups: TaskGroupSummary[];
  pendingStartReference: string | null;
  readOnly: boolean;
}): ReactElement {
  const inputId = useId();
  const [query, setQuery] = useState("");
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const visibleTaskGroups = normalizedQuery
    ? taskGroups.filter(taskGroup =>
        [taskGroup.task_group_id, taskGroup.title, taskGroup.outcome, taskGroup.reference].some(
          value => value.toLocaleLowerCase().includes(normalizedQuery)
        )
      )
    : taskGroups;

  return (
    <section
      aria-label={`Task Groups for ${initiativeSlug}`}
      className="ml-3 border-l border-border pl-4 pt-3 sm:ml-6 sm:pl-6"
      data-testid={`workflow-task-groups-${initiativeSlug}`}
    >
      <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="eyebrow text-muted-foreground">Task Groups · {taskGroups.length}</p>
          <p className="mt-1 text-xs text-muted-foreground">
            Child execution scopes for this initiative.
          </p>
        </div>
        <label className="grid gap-1 text-xs text-muted-foreground" htmlFor={inputId}>
          Filter Task Groups
          <input
            className="h-9 w-full rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 text-sm text-foreground outline-none transition-colors placeholder:text-muted-foreground focus-visible:border-[color:var(--color-primary)] focus-visible:ring-2 focus-visible:ring-[color:var(--focus-ring)] sm:w-64"
            data-testid={`workflow-task-groups-filter-${initiativeSlug}`}
            id={inputId}
            onChange={event => setQuery(event.currentTarget.value)}
            placeholder="ID, title, outcome"
            type="search"
            value={query}
          />
        </label>
      </div>
      {visibleTaskGroups.length > 0 ? (
        <ul className="grid gap-2" data-testid={`workflow-task-groups-list-${initiativeSlug}`}>
          {visibleTaskGroups.map(taskGroup => (
            <TaskGroupRow
              initiativeSlug={initiativeSlug}
              key={taskGroup.workflow_id}
              onStart={request => onStartTaskGroup(request)}
              pendingStart={pendingStartReference === taskGroup.reference}
              taskGroup={taskGroup}
              readOnly={readOnly}
            />
          ))}
        </ul>
      ) : (
        <p
          className="rounded-[var(--radius-md)] border border-dashed border-border px-3 py-4 text-sm text-muted-foreground"
          data-testid={`workflow-task-groups-filter-empty-${initiativeSlug}`}
        >
          No Task Groups match “{query}”.
        </p>
      )}
    </section>
  );
}

function TaskGroupRow({
  initiativeSlug,
  onStart,
  pendingStart,
  taskGroup,
  readOnly,
}: {
  initiativeSlug: string;
  onStart: (request: WorkflowRunRequest) => void | Promise<void>;
  pendingStart: boolean;
  taskGroup: TaskGroupSummary;
  readOnly: boolean;
}): ReactElement {
  const unmetCount = taskGroup.unmet_dependency_count ?? 0;
  const unmetDependencies = taskGroup.unmet_dependencies ?? [];
  const unmetDependencyPaths = taskGroup.unmet_dependency_paths ?? [];
  const requiresStartConfirmation =
    taskGroup.requires_start_confirmation === true || unmetCount > 0;
  const [dependencyConfirmationOpen, setDependencyConfirmationOpen] = useState(false);
  const [dependencyConfirmationPending, setDependencyConfirmationPending] = useState(false);
  const completionText = taskGroup.lifecycle_complete
    ? "Compozy lifecycle complete; Git integration is not tracked"
    : "Compozy lifecycle incomplete";
  const selectionLabel = `${taskGroup.task_group_id}, ${taskGroup.title}. ${completionText}. ${unmetCount} unmet ${unmetCount === 1 ? "dependency" : "dependencies"}.`;
  const canStart =
    !taskGroup.lifecycle_complete &&
    taskGroup.can_start_run !== false &&
    (!requiresStartConfirmation || unmetDependencies.length > 0 || unmetDependencyPaths.length > 0);
  const startBlockReason =
    requiresStartConfirmation && unmetDependencies.length === 0 && unmetDependencyPaths.length === 0
      ? "dependency details unavailable"
      : taskGroup.start_block_reason || "not startable";
  const taskCounts = taskGroup.task_counts;

  async function handleConfirmDependencyOverride() {
    if (dependencyConfirmationPending) return;
    setDependencyConfirmationPending(true);
    try {
      await onStart({ taskGroupId: taskGroup.task_group_id, allowOutOfOrder: true });
      setDependencyConfirmationOpen(false);
    } finally {
      setDependencyConfirmationPending(false);
    }
  }

  return (
    <li>
      <SurfaceCard data-testid={`workflow-task-group-${initiativeSlug}-${taskGroup.task_group_id}`}>
        <SurfaceCardHeader>
          <div className="min-w-0">
            <SurfaceCardEyebrow>{taskGroup.task_group_id}</SurfaceCardEyebrow>
            <SurfaceCardTitle>
              <Link
                aria-label={selectionLabel}
                className="block text-foreground hover:underline"
                data-testid={`workflow-task-group-open-${initiativeSlug}-${taskGroup.task_group_id}`}
                params={{ slug: initiativeSlug }}
                search={{ task_group_id: taskGroup.task_group_id }}
                to="/workflows/$slug/tasks"
              >
                {taskGroup.title}
              </Link>
            </SurfaceCardTitle>
            <SurfaceCardDescription>{taskGroup.outcome}</SurfaceCardDescription>
          </div>
          <StatusBadge
            tone={taskGroup.lifecycle_complete ? "success" : unmetCount > 0 ? "warning" : "info"}
          >
            {taskGroup.lifecycle_complete
              ? "lifecycle complete"
              : unmetCount > 0
                ? "dependencies unmet"
                : "ready"}
          </StatusBadge>
        </SurfaceCardHeader>
        <SurfaceCardBody className="space-y-3">
          <div className="grid gap-2 text-xs text-muted-foreground sm:grid-cols-2 xl:grid-cols-4">
            <p data-testid={`workflow-task-group-lifecycle-${taskGroup.task_group_id}`}>
              {completionText}.
            </p>
            <p data-testid={`workflow-task-group-readiness-${taskGroup.task_group_id}`}>
              {unmetCount > 0
                ? `${pluralize(unmetCount, "unmet dependency")}.`
                : "All declared dependencies are complete."}
            </p>
            <p>
              {taskGroup.independently_eligible
                ? "May be developed independently of an eligible peer."
                : "Follows the declared dependency order."}
            </p>
            <p>
              {taskCounts
                ? `${taskCounts.completed}/${taskCounts.total} tasks complete`
                : "Task counts unavailable"}
              {` · ${pluralize(taskGroup.unresolved_reviews ?? 0, "unresolved review")}`}
              {` · ${pluralize(taskGroup.active_runs ?? 0, "active run")}`}
            </p>
          </div>
          {(taskGroup.dependencies?.length ?? 0) > 0 ? (
            <ul
              aria-label={`Dependencies for ${taskGroup.task_group_id}`}
              className="flex flex-wrap gap-2"
              data-testid={`workflow-task-group-dependencies-${taskGroup.task_group_id}`}
            >
              {taskGroup.dependencies?.map(dependency => (
                <li
                  className="rounded-[var(--radius-sm)] border border-border-subtle bg-[color:var(--surface-inset)] px-2 py-1 text-xs text-muted-foreground"
                  key={`${dependency.task_group_id}-${dependency.rationale}`}
                >
                  <span className="font-mono text-foreground">{dependency.task_group_id}</span>
                  {` — ${dependency.rationale}`}
                </li>
              ))}
            </ul>
          ) : null}
          <div className="flex flex-wrap gap-2">
            <TaskGroupLink
              label="Task board"
              taskGroupId={taskGroup.task_group_id}
              slug={initiativeSlug}
              testId={`workflow-task-group-tasks-${initiativeSlug}-${taskGroup.task_group_id}`}
              to="/workflows/$slug/tasks"
            />
            <TaskGroupLink
              label="Spec + plan"
              taskGroupId={taskGroup.task_group_id}
              slug={initiativeSlug}
              testId={`workflow-task-group-spec-${initiativeSlug}-${taskGroup.task_group_id}`}
              to="/workflows/$slug/spec"
            />
            <TaskGroupLink
              label="Memory"
              taskGroupId={taskGroup.task_group_id}
              slug={initiativeSlug}
              testId={`workflow-task-group-memory-${initiativeSlug}-${taskGroup.task_group_id}`}
              to="/memory/$slug"
            />
            {taskGroup.latest_review ? (
              <Link
                className="inline-flex items-center justify-center rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 py-1.5 text-sm text-foreground transition-colors hover:border-border-strong hover:bg-surface-hover"
                data-testid={`workflow-task-group-reviews-${initiativeSlug}-${taskGroup.task_group_id}`}
                params={{
                  slug: initiativeSlug,
                  round: String(taskGroup.latest_review.round_number),
                }}
                search={{ task_group_id: taskGroup.task_group_id }}
                to="/reviews/$slug/$round"
              >
                {`Reviews · round ${taskGroup.latest_review.round_number}`}
              </Link>
            ) : null}
            {canStart ? (
              <Button
                data-testid={`workflow-task-group-start-${initiativeSlug}-${taskGroup.task_group_id}`}
                disabled={pendingStart || readOnly}
                icon={<Play className="size-4" />}
                loading={pendingStart}
                onClick={() => {
                  if (requiresStartConfirmation) {
                    setDependencyConfirmationOpen(true);
                    return;
                  }
                  void onStart({ taskGroupId: taskGroup.task_group_id });
                }}
                size="sm"
              >
                Start task group run
              </Button>
            ) : taskGroup.lifecycle_complete ? null : (
              <StatusBadge tone="warning">{startBlockReason}</StatusBadge>
            )}
          </div>
        </SurfaceCardBody>
      </SurfaceCard>
      <AlertDialog open={dependencyConfirmationOpen}>
        <AlertDialogContent
          data-testid={`workflow-task-group-dependency-confirmation-${initiativeSlug}-${taskGroup.task_group_id}`}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>Start {taskGroup.reference} out of order?</AlertDialogTitle>
            <AlertDialogDescription>
              This run has unmet dependencies. Continuing authorizes only this task group run and
              does not change the task group plan.
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
              aria-label={`Unmet dependency details for ${taskGroup.task_group_id}`}
              className="space-y-2 rounded-[var(--radius-lg)] border border-border-subtle bg-[color:var(--surface-inset)] px-4 py-3 text-sm text-muted-foreground"
              data-testid={`workflow-task-group-dependency-confirmation-dependencies-${initiativeSlug}-${taskGroup.task_group_id}`}
            >
              {unmetDependencies.map(dependency => (
                <li key={`${dependency.task_group_id}-${dependency.rationale}`}>
                  <span className="font-mono text-foreground">{dependency.task_group_id}</span>
                  {` — ${dependency.title}: ${dependency.rationale}`}
                </li>
              ))}
              {unmetDependencyPaths.map(path => (
                <li key={path.task_group_ids.join("/")}>
                  <p>Transitive path: {path.task_group_ids.join(" → ")}</p>
                  <ul className="mt-1 space-y-1">
                    {path.dependencies.map(dependency => (
                      <li key={`${dependency.task_group_id}-${dependency.rationale}`}>
                        <span className="font-mono text-foreground">
                          {dependency.task_group_id}
                        </span>
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
              data-testid={`workflow-task-group-dependency-confirmation-cancel-${initiativeSlug}-${taskGroup.task_group_id}`}
              disabled={dependencyConfirmationPending}
              onClick={() => setDependencyConfirmationOpen(false)}
              variant="secondary"
            >
              Cancel
            </Button>
            <Button
              data-testid={`workflow-task-group-dependency-confirmation-confirm-${initiativeSlug}-${taskGroup.task_group_id}`}
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

function TaskGroupLink({
  label,
  taskGroupId,
  slug,
  testId,
  to,
}: {
  label: string;
  taskGroupId: string;
  slug: string;
  testId: string;
  to: "/memory/$slug" | "/workflows/$slug/spec" | "/workflows/$slug/tasks";
}): ReactElement {
  return (
    <Link
      className="inline-flex items-center justify-center rounded-[var(--radius-md)] border border-border bg-[color:var(--surface-inset)] px-3 py-1.5 text-sm text-foreground transition-colors hover:border-border-strong hover:bg-surface-hover"
      data-testid={testId}
      params={{ slug }}
      search={{ task_group_id: taskGroupId }}
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
      {(workflow.task_groups?.length ?? 0) > 0 ? (
        <ul
          aria-label={`Archived Task Groups for ${workflow.slug}`}
          className="ml-3 grid gap-2 border-l border-border pl-4 pt-3 sm:ml-6 sm:pl-6"
        >
          {workflow.task_groups?.map(taskGroup => (
            <li
              className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)] px-3 py-2"
              key={taskGroup.workflow_id}
            >
              <p className="font-mono text-xs text-muted-foreground">{taskGroup.task_group_id}</p>
              <p className="mt-1 text-sm font-medium text-foreground">{taskGroup.title}</p>
              <p className="mt-1 text-xs text-muted-foreground">
                {taskGroup.lifecycle_complete
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
