import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import { validateTasksSearch, type TasksRouteSearch } from "@/systems/tasks";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadTasksRoute } from "./-tasks-preload";

export type TasksSurfaceMode = "list" | "kanban" | "dashboard" | "inbox";
export type { TasksRouteSearch };

export const Route = createFileRoute("/_app/tasks")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Tasks", to: "/tasks" } },
  }),
  validateSearch: validateTasksSearch,
  loaderDeps: ({ search }) => ({ mode: search.mode }),
  loader: ({ context, deps }) => preloadTasksRoute(context.queryClient, deps.mode),
  component: createOsRouteSync("tasks"),
});
