import { Link } from "@tanstack/react-router";
import { AlertCircle, ListChecks, Plus } from "lucide-react";

import { BlockLoading, Button, Empty, ListingPage, RouteNav, useTopbarSlot } from "@compozy/ui";

import { useTasksCatalogLocation } from "./hooks/use-tasks-catalog-location";
import {
  taskModeSearchFor,
  TasksDashboardView,
  TasksEmptyState,
  TasksInboxView,
  TasksKanbanBoard,
  TasksListSurface,
  TasksListToolbar,
  type TasksRouteSearch,
  type TaskViewMode,
} from "@/systems/tasks";

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

export function TasksCatalogLocation({ search }: { search: TasksRouteSearch }) {
  const { localTaskLifecycle, mode, navigate, openCreate, page } = useTasksCatalogLocation({
    search,
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
              search={taskModeSearchFor(item.value, search)}
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
    actions: localTaskLifecycle ? (
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
    ) : null,
    // Route views lead the strip on every route (ADR-007/D3): the head stays
    // two-element, strip order views · filters · spacer · display-mode.
    toolbar: (
      <>
        {modeNav}
        {mode === "list" && page.hasActiveTaskScope ? (
          <TasksListToolbar
            onOwnerChange={page.handleOwnerChange}
            onPriorityChange={page.handlePriorityChange}
            onRecordsFilterChange={page.handleRecordsFilterChange}
            onSearchQueryChange={page.setSearchQuery}
            onSortChange={page.handleSortChange}
            onStatusChange={page.handleStatusChange}
            ownerFilter={page.ownerFilter}
            ownerOptions={page.ownerOptions}
            priorityFilter={page.priorityFilter}
            recordsFilter={page.recordsFilter}
            searchQuery={page.searchQuery}
            sortBy={page.sortBy}
            statusFilter={page.statusFilter}
          />
        ) : null}
      </>
    ),
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
          schedulerAvailable={page.schedulerAvailable}
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
          // Triage and run mutations are local-only lifecycle routes; without
          // their handlers the rows render read-only (absent, not disabled).
          onApprove={localTaskLifecycle ? page.handleApproveTask : undefined}
          onArchive={localTaskLifecycle ? page.handleArchiveTask : undefined}
          onDismiss={localTaskLifecycle ? page.handleDismissTask : undefined}
          onLaneChange={page.handleInboxLaneChange}
          onMarkRead={localTaskLifecycle ? page.handleMarkTaskRead : undefined}
          onLoadMore={page.loadMoreInbox}
          onPriorityChange={page.handleInboxPriorityChange}
          onReject={localTaskLifecycle ? page.handleRejectTask : undefined}
          onRetry={localTaskLifecycle ? page.handleRetryRun : undefined}
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
        <ListingPage data-testid="tasks-list-surface">
          {localTaskLifecycle ? (
            <TasksEmptyState
              onSelectTemplate={openCreate}
              profileScopeLabel={page.profile.scopeLabel}
              workspaceName={page.activeWorkspaceName}
            />
          ) : (
            <Empty
              data-testid="tasks-list-remote-empty"
              description="Create or run tasks from the machine running CompozyOS."
              icon={ListChecks}
              title="No tasks yet"
            />
          )}
        </ListingPage>
      ) : mode === "kanban" ? (
        <TasksKanbanBoard
          columns={page.kanbanColumns}
          errorMessage={page.listError?.message ?? null}
          hasMore={page.hasMoreTasks}
          isLoading={page.listLoading}
          isLoadingMore={page.isLoadingMoreTasks}
          onCreate={localTaskLifecycle ? () => openCreate() : undefined}
          onRetryLoad={page.retryTasks}
          // Run retry hits POST /api/runs/:id/retry, a local-only lifecycle
          // route: the affordance goes absent on remote tiers, like the inbox
          // triage handlers above (BR-1 — absent, never disabled).
          onRetryTask={localTaskLifecycle ? page.handleRetryRun : undefined}
          onSelectTask={taskId => navigate(`/tasks/${encodeURIComponent(taskId)}`)}
          onLoadMore={page.loadMoreTasks}
          profile={page.profile}
          selectedTaskId={page.effectiveSelectedTaskId}
          statusCounts={page.statusCounts}
        />
      ) : (
        <TasksListSurface
          profile={page.profile}
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
          onOpenLoopRun={() => page.handleRecordsFilterChange("work")}
          onShowWorkItems={() => page.handleRecordsFilterChange("work")}
          recordsFilter={page.recordsFilter}
          searchQuery={page.searchQuery}
          statusCounts={page.statusCounts}
          tasks={page.visibleTasks}
        />
      )}
    </div>
  );
}
