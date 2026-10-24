import { groupTasksForKanban } from "../lib/task-grouping";
import { taskStatusCountsFromFacets } from "../lib/task-list-query";
import type { TaskViewMode } from "../types";
import { TasksDashboardView } from "./tasks-dashboard-view";
import { TasksInboxViewSkeleton } from "./tasks-inbox-view";
import { TasksKanbanBoard } from "./tasks-kanban-board";
import { TasksListLoading } from "./tasks-list-surface";

const noop = () => undefined;

/**
 * The catalog before its first read has answered, in the shape of the view the window is on: the
 * table, the inbox groups, the board's columns or the dashboard's sections, each the loading state
 * its own view shows. Nothing here reads a query or takes focus, so a window can show it before
 * its code has loaded. The view's own status region is the only one.
 */
export function TasksCatalogSkeleton({ mode }: { mode: TaskViewMode }) {
  if (mode === "dashboard")
    return <TasksDashboardView dashboard={null} dashboardStatus="loading" />;
  if (mode === "inbox") return <TasksInboxViewSkeleton />;
  if (mode === "kanban") {
    return (
      <TasksKanbanBoard
        columns={groupTasksForKanban([])}
        isLoading
        onSelectTask={noop}
        selectedTaskId={null}
        statusCounts={taskStatusCountsFromFacets({ owners: [], statuses: [] })}
      />
    );
  }
  return <TasksListLoading />;
}
