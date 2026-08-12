import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name/configure")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Configure" } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-loops-preload")).preloadLoopConfigureRoute(context.queryClient, params.name),
  component: createOsRouteSync("loops"),
});
