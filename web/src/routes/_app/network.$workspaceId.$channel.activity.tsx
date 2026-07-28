import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadNetworkActivityRoute } from "./-network-preload";

export const Route = createFileRoute("/_app/network/$workspaceId/$channel/activity")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: `#${params.channel} · Activity` } },
  }),
  component: createOsRouteSync("network"),
  loader: ({ context, params }) =>
    preloadNetworkActivityRoute(context.queryClient, params.workspaceId, params.channel),
});
