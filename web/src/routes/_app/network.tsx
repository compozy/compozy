import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadNetworkRootRoute } from "./-network-preload";

export const Route = createFileRoute("/_app/network")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Network", to: "/network" } },
  }),
  loader: ({ context }) => preloadNetworkRootRoute(context.queryClient),
  component: createOsRouteSync("network"),
});
