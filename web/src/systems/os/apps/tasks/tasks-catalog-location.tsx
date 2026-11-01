import { Link, useNavigate } from "@tanstack/react-router";
import { ListChecks, Plus } from "@compozy/ui/icons";

import { Button, RouteNav, Separator, useTopbarSlot } from "@compozy/ui";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { TasksCatalogContent } from "./tasks-catalog-views";
import { useTasksCatalogNavigation } from "./use-tasks-catalog-navigation";
import {
  taskModeSearchFor,
  TasksListToolbar,
  type TasksRouteSearch,
  type TaskViewMode,
  useTasksPage,
  validateTasksSearch,
} from "@/systems/tasks";

const TASK_MODE_ITEMS: ReadonlyArray<{
  value: TaskViewMode;
  label: string;
  testId: string;
}> = [
  { value: "list", label: "List", testId: "tasks-mode-list" },
  { value: "inbox", label: "Inbox", testId: "tasks-mode-inbox" },
  { value: "kanban", label: "Kanban", testId: "tasks-mode-kanban" },
  { value: "dashboard", label: "Dashboard", testId: "tasks-mode-dashboard" },
];

export function TasksCatalogLocation({ search }: { search: TasksRouteSearch }) {
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const routeNavigate = useNavigate();
  const mode: TaskViewMode = search.mode ?? "list";
  const page = useTasksPage({
    liveDataEnabled,
    search,
    onSearchChange: update => {
      void routeNavigate({
        to: "/tasks",
        search: current => update(validateTasksSearch(current)),
        replace: true,
      });
    },
  });
  const { openCreate, openTask } = useTasksCatalogNavigation({
    mode,
    search,
    tasks: page.visibleTasks,
  });
  const modeNav = (
    <TasksModeNav inboxUnreadCount={page.inboxUnreadCount} mode={mode} search={search} />
  );

  useTopbarSlot({
    glyph: <ListChecks />,
    // The head count names the whole list; under a filter the table header
    // carries the (filtered) count instead, so the head never implies a total
    // it is not showing.
    count:
      mode === "list" && !page.listLoading && !page.hasListFilters ? page.tasksCount : undefined,
    actions: (
      <Button
        data-testid="tasks-open-create"
        disabled={!page.hasActiveTaskScope}
        onClick={() => openCreate()}
        size="sm"
        type="button"
        variant="secondary"
      >
        <Plus aria-hidden="true" data-icon="inline-start" />
        New task
      </Button>
    ),
    // Route views lead the strip on every route (ADR-007/D3): the head stays
    // two-element, strip order views · filters · spacer · display-mode.
    toolbar: (
      <>
        {modeNav}
        {/* The strip needs no data: it renders while the scope resolves, so it does not pop in. */}
        {mode === "list" && !page.scopeError ? (
          <>
            <Separator className="h-4 self-center" orientation="vertical" />
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
              projectFilter={page.projectFilter}
              recordsFilter={page.recordsFilter}
              searchQuery={page.searchQuery}
              sortBy={page.sortBy}
              statusCounts={page.statusCounts}
              statusCountsLoading={page.listLoading}
              statusFilter={page.statusFilter}
            />
          </>
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
      <TasksCatalogContent mode={mode} openCreate={openCreate} openTask={openTask} page={page} />
    </div>
  );
}

function TasksModeNav({
  mode,
  search,
  inboxUnreadCount,
}: {
  mode: TaskViewMode;
  search: TasksRouteSearch;
  inboxUnreadCount?: number;
}) {
  return (
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
          {item.value === "inbox" && inboxUnreadCount ? (
            <RouteNav.Count data-testid="tasks-mode-inbox-count">{inboxUnreadCount}</RouteNav.Count>
          ) : null}
        </RouteNav.Link>
      ))}
    </RouteNav>
  );
}
