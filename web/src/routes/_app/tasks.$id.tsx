import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { validateTaskDetailSearch } from "@/systems/tasks";

export const Route = createFileRoute("/_app/tasks/$id")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: {
      crumb: { label: `Task ${params.id}`, params: { id: params.id }, to: "/tasks/$id" },
    },
  }),
  validateSearch: validateTaskDetailSearch,
  loader: async ({ context, params }) =>
    (await import("./-tasks-preload")).preloadTaskDetailRoute(context.queryClient, params.id),
  component: createOsRouteSync("tasks"),
});
