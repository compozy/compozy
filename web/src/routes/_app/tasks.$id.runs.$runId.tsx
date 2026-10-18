import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/tasks/$id/runs/$runId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `Run ${params.runId}` } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-tasks-preload")).preloadTaskRunRoute(
      context.queryClient,
      params.id,
      params.runId
    ),
  component: createOsRouteSync("tasks"),
});
