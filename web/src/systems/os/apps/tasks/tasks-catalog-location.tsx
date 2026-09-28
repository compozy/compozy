import { Link, useNavigate } from "@tanstack/react-router";
import { ListChecks, Plus } from "lucide-react";

import { Button, RouteNav, useTopbarSlot } from "@compozy/ui";

import { useOsShell } from "../../hooks/use-os-shell";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { TasksCatalogBody } from "./tasks-catalog-views";
import {
  DEFAULT_TASK_TEMPLATE_ID,
  taskCatalogSearchFor,
  taskModeSearchFor,
  TasksListToolbar,
  type TasksRouteSearch,
  type TaskTemplateId,
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
  const { coordinator } = useOsShell();
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
  const navigate = (pathname: string, search: Record<string, unknown> = {}) =>
    void coordinator.userOpen({ app: "tasks", route: { pathname, search } });
  // The create dialog layers over this catalog, so the active view rides along
  // and dismissal lands back on the view the operator was reading.
  const openCreate = (template?: TaskTemplateId) =>
    navigate("/tasks/new", {
      ...taskCatalogSearchFor(mode, search),
      ...(template && template !== DEFAULT_TASK_TEMPLATE_ID ? { template } : {}),
    });
  const modeNav = (
    <TasksModeNav inboxUnreadCount={page.inboxUnreadCount} mode={mode} search={search} />
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
      <TasksCatalogBody
        mode={mode}
        openCreate={openCreate}
        openTask={taskId => navigate(`/tasks/${encodeURIComponent(taskId)}`)}
        page={page}
      />
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
