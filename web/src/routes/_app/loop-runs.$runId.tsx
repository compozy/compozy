import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loop-runs/$runId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.runId } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-loops-preload")).preloadLoopRunDetailRoute(context.queryClient, params.runId),
  component: createOsRouteSync("loops"),
});
