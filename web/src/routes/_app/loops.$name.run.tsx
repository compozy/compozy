import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopRunFormRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loops/$name/run")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Run" } },
  }),
  loader: ({ context, params }) => preloadLoopRunFormRoute(context.queryClient, params.name),
  component: createOsRouteSync("loops"),
});
