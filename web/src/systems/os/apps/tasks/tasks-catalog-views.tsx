import { AlertCircle } from "lucide-react";

import { BlockLoading, Empty, ListingPage } from "@compozy/ui";

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

/** Active catalog view once the task scope resolves; scope gates lead, then the mode. */
export function TasksCatalogBody({ mode, page, openCreate, openTask }: TasksCatalogBodyProps) {
  if (page.scopeLoading) {
    return (
      <BlockLoading
        data-testid="tasks-scope-loading"
        label="Loading tasks…"
        size="md"
        surface="bare"
      />
    );
  }
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
    return (
      <ListingPage data-testid="tasks-list-surface">
        <TasksEmptyState
          onSelectTemplate={openCreate}
          profileScopeLabel={page.profile.scopeLabel}
          workspaceName={page.activeWorkspaceName}
        />
      </ListingPage>
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
      searchQuery={page.inboxSearchQuery}
      statusFilter={page.inboxStatusFilter}
      unreadOnly={page.inboxUnreadOnly}
      pendingApproveIds={page.pendingApproveIds}
      pendingArchiveIds={page.pendingArchiveIds}
      pendingDismissIds={page.pendingDismissIds}
      pendingMarkReadIds={page.pendingMarkReadIds}
      pendingRejectIds={page.pendingRejectIds}
      pendingRetryIds={page.pendingRetryIds}
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
      selectedTaskId={page.effectiveSelectedTaskId}
      statusCounts={page.statusCounts}
    />
  );
}

function listFilterState(page: TasksPageModel): "active" | "inactive" {
  const filtered = [page.statusFilter, page.ownerFilter, page.priorityFilter].some(Boolean);
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
      onLoadMore={page.loadMoreTasks}
      onRetryLoad={page.retryTasks}
      onOpenLoopRun={showWorkItems}
      onShowWorkItems={showWorkItems}
      recordsFilter={page.recordsFilter}
      searchQuery={page.searchQuery}
      statusCounts={page.statusCounts}
      tasks={page.visibleTasks}
    />
  );
}
