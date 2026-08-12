import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/activity")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `#${params.channel} · Activity` } },
  }),
  component: createOsRouteSync("network"),
  loader: async ({ context, params }) =>
    (await import("./-network-preload")).preloadNetworkActivityRoute(
      context.queryClient,
      params.workspaceId,
      params.channel
    ),
});
