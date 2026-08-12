import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/network")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Network", to: "/network" } },
  }),
  loader: async ({ context }) =>
    (await import("./-network-preload")).preloadNetworkRootRoute(context.queryClient),
  component: createOsRouteSync("network"),
});
