import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/directs")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: {
      crumb: {
        label: `#${params.channel} · Directs`,
        params: { channel: params.channel, workspaceId: params.workspaceId },
        to: "/network/$workspaceId/$channel/directs",
      },
    },
  }),
  component: createOsRouteSync("network"),
  loader: async ({ context, params }) =>
    (await import("./-network-preload")).preloadNetworkDirectsRoute(
      context.queryClient,
      params.workspaceId,
      params.channel
    ),
});
