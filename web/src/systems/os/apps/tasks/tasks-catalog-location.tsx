import { Link } from "@tanstack/react-router";
import { AlertCircle, ListChecks, Plus } from "lucide-react";

import { BlockLoading, Button, Empty, RouteNav, useTopbarSlot } from "@agh/ui";

import {
  DEFAULT_TASK_TEMPLATE_ID,
  TasksDashboardView,
  TasksEmptyState,
  TasksInboxView,
  TasksKanbanBoard,
  TasksListSurface,
  TasksListToolbar,
  taskCatalogSearchFor,
  useTasksPage,
  type TaskTemplateId,
  type TaskViewMode,
} from "@/systems/tasks";

import { useOsShell } from "../../hooks/use-os-shell";

const TASK_MODE_ITEMS: ReadonlyArray<{
  value: TaskViewMode;
  label: string;
  testId: string;
}> = [
  { value: "list", label: "List", testId: "tasks-mode-list" },
  { value: "kanban", label: "Kanban", testId: "tasks-mode-kanban" },
  { value: "dashboard", label: "Dashboard", testId: "tasks-mode-dashboard" },
  { value: "inbox", label: "Inbox", testId: "tasks-mode-inbox" },
];

export function TasksCatalogLocation({ mode }: { mode: TaskViewMode }) {
  const { coordinator } = useOsShell();
  const page = useTasksPage({ mode });
  const navigate = (pathname: string, search: Record<string, unknown> = {}) =>
    void coordinator.userOpen({ app: "tasks", route: { pathname, search } });
  // The create dialog layers over this catalog, so the active view rides along
  // and dismissal lands back on the view the operator was reading.
  const openCreate = (template?: TaskTemplateId) =>
    navigate("/tasks/new", {
      ...taskCatalogSearchFor(mode),
      ...(template && template !== DEFAULT_TASK_TEMPLATE_ID ? { template } : {}),
    });
  const modeNav = (
    <RouteNav aria-label="Tasks views" data-testid="tasks-mode-nav">
      {TASK_MODE_ITEMS.map(item => (
        <RouteNav.Link
          aria-current={item.value === mode ? "page" : undefined}
          data-testid={item.testId}
          key={item.value}
          render={
            <Link
              activeOptions={{ exact: true, includeSearch: true }}
              onClick={() => {
                if (item.value !== mode) page.setSearchQuery("");
              }}
              search={item.value === "list" ? {} : { mode: item.value }}
              to="/tasks"
            />
          }
        >
          {item.label}
          {item.value === "inbox" && page.inboxUnreadCount ? (
            <RouteNav.Count data-testid="tasks-mode-inbox-count">
              {page.inboxUnreadCount}
            </RouteNav.Count>
          ) : null}
        </RouteNav.Link>
      ))}
    </RouteNav>
  );

  useTopbarSlot({
    glyph: <ListChecks />,
    count: mode === "list" && !page.listLoading ? page.tasksCount : undefined,
    actions: (
      <Button
        data-testid="tasks-open-create"
        disabled={!page.hasActiveTaskScope}
        onClick={() => openCreate()}
        size="sm"
        type="button"
      >
        <Plus className="size-3" />
        New task
      </Button>
    ),
    nav: modeNav,
    toolbar:
      mode === "list" && page.hasActiveTaskScope ? (
        <TasksListToolbar
          onOwnerChange={page.handleOwnerChange}
          onPriorityChange={page.handlePriorityChange}
          onSearchQueryChange={page.setSearchQuery}
          onSortChange={page.handleSortChange}
          onStatusChange={page.handleStatusChange}
          ownerFilter={page.ownerFilter}
          ownerOptions={page.ownerOptions}
          priorityFilter={page.priorityFilter}
          searchQuery={page.searchQuery}
          sortBy={page.sortBy}
          statusFilter={page.statusFilter}
        />
      ) : undefined,
  });

  return (
    <div
      className="flex min-h-0 flex-1 flex-col overflow-hidden"
      data-density="route"
      data-testid="tasks-shell"
    >
      {page.scopeLoading ? (
        <BlockLoading
          data-testid="tasks-scope-loading"
          label="Resolving task scope"
          size="md"
          surface="bare"
        />
      ) : page.scopeError ? (
        <Empty
          data-testid="tasks-scope-error"
          description={page.scopeError.message}
          icon={AlertCircle}
          title="Unable to resolve task scope"
        />
      ) : mode === "dashboard" ? (
        <TasksDashboardView
          dashboard={page.dashboard}
          errorMessage={page.dashboardError?.message ?? null}
          dashboardStatus={page.dashboardLoading ? "loading" : "ready"}
          scheduler={page.schedulerStatus}
          schedulerBacklog={page.schedulerBacklog}
          schedulerBacklogErrorMessage={page.schedulerBacklogError?.message ?? null}
          schedulerBacklogStatus={page.schedulerBacklogLoading ? "loading" : "ready"}
          schedulerErrorMessage={page.schedulerStatusError?.message ?? null}
          schedulerStatus={page.schedulerStatusLoading ? "loading" : "ready"}
          schedulerPendingActions={
            new Set(
              [
                page.isSchedulerDrainPending ? "drain" : null,
                page.isSchedulerPausePending ? "pause" : null,
                page.isSchedulerResumePending ? "resume" : null,
              ].filter((action): action is "drain" | "pause" | "resume" => action !== null)
            )
          }
          onDrainScheduler={page.handleDrainScheduler}
          onPauseScheduler={page.handlePauseScheduler}
          onResumeScheduler={page.handleResumeScheduler}
        />
      ) : mode === "inbox" ? (
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
      ) : page.isEmpty ? (
        <TasksEmptyState onSelectTemplate={openCreate} workspaceName={page.activeWorkspaceName} />
      ) : mode === "kanban" ? (
        <TasksKanbanBoard
          columns={page.kanbanColumns}
          errorMessage={page.listError?.message ?? null}
          hasMore={page.hasMoreTasks}
          isLoading={page.listLoading}
          isLoadingMore={page.isLoadingMoreTasks}
          onCreate={() => openCreate()}
          onRetryLoad={page.retryTasks}
          onRetryTask={page.handleRetryRun}
          onSelectTask={taskId => navigate(`/tasks/${encodeURIComponent(taskId)}`)}
          onLoadMore={page.loadMoreTasks}
          selectedTaskId={page.effectiveSelectedTaskId}
          statusCounts={page.statusCounts}
        />
      ) : (
        <TasksListSurface
          errorMessage={page.listError?.message ?? null}
          filterState={
            Boolean(page.statusFilter) ||
            Boolean(page.ownerFilter) ||
            Boolean(page.priorityFilter) ||
            page.searchQuery.trim() !== ""
              ? "active"
              : "inactive"
          }
          isLoading={page.listLoading}
          hasMore={page.hasMoreTasks}
          isLoadingMore={page.isLoadingMoreTasks}
          onLoadMore={page.loadMoreTasks}
          onRetryLoad={page.retryTasks}
          searchQuery={page.searchQuery}
          statusCounts={page.statusCounts}
          tasks={page.visibleTasks}
        />
      )}
    </div>
  );
}
