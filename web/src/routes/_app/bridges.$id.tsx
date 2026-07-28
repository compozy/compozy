import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadBridgeDetailRoute } from "./-bridges-preload";

export const Route = createFileRoute("/_app/bridges/$id")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.id } },
  }),
  loader: ({ context, params }) => preloadBridgeDetailRoute(context.queryClient, params.id),
  component: createOsRouteSync("bridges"),
});
