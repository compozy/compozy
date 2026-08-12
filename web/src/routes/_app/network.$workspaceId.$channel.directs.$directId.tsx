import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/directs/$directId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `#${params.channel} · Direct` } },
  }),
  component: createOsRouteSync("network"),
  loader: async ({ context, params }) =>
    (await import("./-network-preload")).preloadNetworkDirectDetailRoute(
      context.queryClient,
      params.workspaceId,
      params.channel,
      params.directId
    ),
});
