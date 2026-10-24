import type { TaskViewMode } from "../types";
import { TasksDashboardLoading } from "./tasks-dashboard-loading";
import { TasksInboxViewSkeleton } from "./tasks-inbox-loading";
import { TasksKanbanBoardSkeleton } from "./tasks-kanban-skeleton";
import { TasksListLoading } from "./tasks-list-loading";

/**
 * The catalog before its first read has answered, in the shape of the view the window is on: the
 * table, the inbox groups, the board's columns or the dashboard's sections, each the loading state
 * its own view shows. Nothing here reads a query or takes focus, so a window can show it before
 * its code has loaded. The view's own status region is the only one.
 */
export function TasksCatalogSkeleton({ mode }: { mode: TaskViewMode }) {
  if (mode === "dashboard") return <TasksDashboardLoading />;
  if (mode === "inbox") return <TasksInboxViewSkeleton />;
  if (mode === "kanban") return <TasksKanbanBoardSkeleton />;
  return <TasksListLoading />;
}
