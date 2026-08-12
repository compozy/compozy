import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { type TasksRouteSearch, validateTasksSearch } from "@/systems/tasks";

export type TasksSurfaceMode = "list" | "kanban" | "dashboard" | "inbox";
export type { TasksRouteSearch };

export const Route = createFileRoute("/_app/tasks")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Tasks", to: "/tasks" } },
  }),
  validateSearch: validateTasksSearch,
  loaderDeps: ({ search }) => search,
  loader: async ({ context, deps }) =>
    (await import("./-tasks-preload")).preloadTasksRoute(context.queryClient, deps),
  component: createOsRouteSync("tasks"),
});
