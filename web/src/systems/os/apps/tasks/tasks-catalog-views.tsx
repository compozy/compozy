import { AlertCircle } from "lucide-react";
import { useDeferredValue, ViewTransition } from "react";

import { Empty, VIEW_TRANSITION_CLASS } from "@compozy/ui";

import {
  TasksDashboardView,
  TasksEmptyState,
  TasksInboxView,
  TasksKanbanBoard,
  TasksListSurface,
  type TaskTemplateId,
  type TaskViewMode,
  type useTasksPage,
} from "@/systems/tasks";

type TasksPageModel = ReturnType<typeof useTasksPage>;
type SchedulerAction = "drain" | "pause" | "resume";

export interface TasksCatalogBodyProps {
  mode: TaskViewMode;
  page: TasksPageModel;
  openCreate: (template?: TaskTemplateId) => void;
  openTask: (taskId: string) => void;
}

/**
 * The catalog body, with a mode swap cross-faded: keyed by mode so only the swap triggers it and
 * never a data update inside one view. The mode arrives from the window-manager store, whose
 * updates are never React transitions, so the body reads it deferred: the strip highlights the
 * new view at once and the swap renders as the transition the boundary animates. Push and pop
 * navigations keep the window manager's own drill, so no surface-level boundary sits here.
 */
export function TasksCatalogContent({ mode, ...bodyProps }: TasksCatalogBodyProps) {
  const bodyMode = useDeferredValue(mode);
  return (
    <ViewTransition
      key={bodyMode}
      default="none"
      enter={VIEW_TRANSITION_CLASS.fade}
      exit={VIEW_TRANSITION_CLASS.fade}
    >
      <TasksCatalogBody mode={bodyMode} {...bodyProps} />
    </ViewTransition>
  );
}

/**
 * Active catalog view; a scope that cannot resolve leads, then the mode. A scope still resolving
 * is not a gate of its own: every mode folds it into its loading flag and shows its own
 * content-shaped skeleton in its own container.
 */
export function TasksCatalogBody({ mode, page, openCreate, openTask }: TasksCatalogBodyProps) {
  if (page.scopeError) {
    return (
      <Empty
        data-testid="tasks-scope-error"
        description={page.scopeError.message}
        icon={AlertCircle}
        title="Couldn't load tasks for this project"
      />
    );
  }
  if (mode === "dashboard") return <TasksDashboardMode page={page} />;
  if (mode === "inbox") return <TasksInboxMode page={page} />;
  if (page.isEmpty) {
    // The empty state is its own full-height scroll surface; a ListingPage
    // around it would nest a second scroller and pad its bottom away.
    return (
      <div className="flex min-h-0 flex-1 flex-col" data-testid="tasks-list-surface">
        <TasksEmptyState
          onSelectTemplate={openCreate}
          profileScopeLabel={page.profile.scopeLabel}
          workspaceName={page.activeWorkspaceName}
        />
      </div>
    );
  }
  if (mode === "kanban") {
    return <TasksKanbanMode onCreate={() => openCreate()} onSelectTask={openTask} page={page} />;
  }
  return <TasksListMode page={page} />;
}

function loadStatus(loading: boolean): "loading" | "ready" {
  return loading ? "loading" : "ready";
}

function schedulerPendingActions(page: TasksPageModel): ReadonlySet<SchedulerAction> {
  const pending: Array<[boolean, SchedulerAction]> = [
    [page.isSchedulerDrainPending, "drain"],
    [page.isSchedulerPausePending, "pause"],
    [page.isSchedulerResumePending, "resume"],
  ];
  return new Set(pending.filter(([isPending]) => isPending).map(([, action]) => action));
}

function TasksDashboardMode({ page }: { page: TasksPageModel }) {
  return (
    <TasksDashboardView
      dashboard={page.dashboard}
      errorMessage={page.dashboardError?.message ?? null}
      dashboardStatus={loadStatus(page.dashboardLoading)}
      scheduler={page.schedulerStatus}
      schedulerBacklog={page.schedulerBacklog}
      schedulerBacklogErrorMessage={page.schedulerBacklogError?.message ?? null}
      schedulerBacklogStatus={loadStatus(page.schedulerBacklogLoading)}
      schedulerErrorMessage={page.schedulerStatusError?.message ?? null}
      schedulerStatus={loadStatus(page.schedulerStatusLoading)}
      schedulerPendingActions={schedulerPendingActions(page)}
      onDrainScheduler={page.handleDrainScheduler}
      onPauseScheduler={page.handlePauseScheduler}
      onResumeScheduler={page.handleResumeScheduler}
      projectBadgeOf={page.projectBadgeOf}
    />
  );
}

function TasksInboxMode({ page }: { page: TasksPageModel }) {
  return (
    <TasksInboxView
      errorMessage={page.inboxError?.message ?? null}
      inbox={page.inbox}
      hasMore={page.hasMoreInbox}
      isLoading={page.inboxLoading}
      isLoadingMore={page.isLoadingMoreInbox}
      laneFilter={page.inboxLaneFilter}
      onApprove={page.handleApproveTask}
      onArchive={page.handleArchiveTask}
      onDismiss={page.handleDismissTask}
      onLaneChange={page.handleInboxLaneChange}
      onMarkRead={page.handleMarkTaskRead}
      onLoadMore={page.loadMoreInbox}
      onPriorityChange={page.handleInboxPriorityChange}
      onReject={page.handleRejectTask}
      onRetry={page.handleRetryRun}
      onRetryQuery={page.retryInbox}
      onSearchChange={page.setInboxSearchQuery}
      onStatusChange={page.handleInboxStatusChange}
      onToggleUnread={page.handleInboxUnreadToggle}
      priorityFilter={page.inboxPriorityFilter}
      projectFilter={page.projectFilter}
      searchQuery={page.inboxSearchQuery}
      statusFilter={page.inboxStatusFilter}
      unreadOnly={page.inboxUnreadOnly}
      pendingApproveIds={page.pendingApproveIds}
      pendingArchiveIds={page.pendingArchiveIds}
      pendingDismissIds={page.pendingDismissIds}
      pendingMarkReadIds={page.pendingMarkReadIds}
      pendingRejectIds={page.pendingRejectIds}
      pendingRetryIds={page.pendingRetryIds}
      projectBadgeOf={page.projectBadgeOf}
    />
  );
}

function TasksKanbanMode({
  page,
  onCreate,
  onSelectTask,
}: {
  page: TasksPageModel;
  onCreate: () => void;
  onSelectTask: (taskId: string) => void;
}) {
  return (
    <TasksKanbanBoard
      columns={page.kanbanColumns}
      errorMessage={page.listError?.message ?? null}
      hasMore={page.hasMoreTasks}
      isLoading={page.listLoading}
      isLoadingMore={page.isLoadingMoreTasks}
      onCreate={onCreate}
      onRetryLoad={page.retryTasks}
      onRetryTask={page.handleRetryRun}
      onSelectTask={onSelectTask}
      onLoadMore={page.loadMoreTasks}
      profile={page.profile}
      projectBadgeOf={page.projectBadgeOf}
      selectedTaskId={page.effectiveSelectedTaskId}
      statusCounts={page.statusCounts}
    />
  );
}

function listFilterState(page: TasksPageModel): "active" | "inactive" {
  const filtered =
    [page.statusFilter, page.ownerFilter, page.priorityFilter].some(Boolean) ||
    page.projectFilter.values.length > 0;
  return filtered || page.searchQuery.trim() !== "" ? "active" : "inactive";
}

function TasksListMode({ page }: { page: TasksPageModel }) {
  const showWorkItems = () => page.handleRecordsFilterChange("work");
  return (
    <TasksListSurface
      profile={page.profile}
      errorMessage={page.listError?.message ?? null}
      filterState={listFilterState(page)}
      isLoading={page.listLoading}
      hasMore={page.hasMoreTasks}
      isLoadingMore={page.isLoadingMoreTasks}
      onClearFilters={page.clearListFilters}
      onLoadMore={page.loadMoreTasks}
      onRetryLoad={page.retryTasks}
      onOpenLoopRun={showWorkItems}
      projectBadgeOf={page.projectBadgeOf}
      onShowWorkItems={showWorkItems}
      recordsFilter={page.recordsFilter}
      searchQuery={page.searchQuery}
      statusCounts={page.statusCounts}
      tasks={page.visibleTasks}
    />
  );
}
