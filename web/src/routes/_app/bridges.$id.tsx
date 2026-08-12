import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/bridges/$id")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.id } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-bridges-preload")).preloadBridgeDetailRoute(context.queryClient, params.id),
  component: createOsRouteSync("bridges"),
});
