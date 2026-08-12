import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name/run")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Run" } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-loops-preload")).preloadLoopRunFormRoute(context.queryClient, params.name),
  component: createOsRouteSync("loops"),
});
