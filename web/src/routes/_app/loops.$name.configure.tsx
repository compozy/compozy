import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopConfigureRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loops/$name/configure")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Configure" } },
  }),
  loader: ({ context, params }) => preloadLoopConfigureRoute(context.queryClient, params.name),
  component: createOsRouteSync("loops"),
});
