import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopRunDetailRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loop-runs/$runId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.runId } },
  }),
  loader: ({ context, params }) => preloadLoopRunDetailRoute(context.queryClient, params.runId),
  component: createOsRouteSync("loops"),
});
