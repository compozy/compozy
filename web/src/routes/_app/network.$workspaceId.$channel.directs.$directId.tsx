import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadNetworkDirectDetailRoute } from "./-network-preload";

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/directs/$directId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `#${params.channel} · Direct` } },
  }),
  component: createOsRouteSync("network"),
  loader: ({ context, params }) =>
    preloadNetworkDirectDetailRoute(
      context.queryClient,
      params.workspaceId,
      params.channel,
      params.directId
    ),
});
