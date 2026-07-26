import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadNetworkDirectsRoute } from "./-network-preload";

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
  loader: ({ context, params }) =>
    preloadNetworkDirectsRoute(context.queryClient, params.workspaceId, params.channel),
});
