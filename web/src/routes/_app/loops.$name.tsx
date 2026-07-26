import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopDetailRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loops/$name")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.name, params: { name: params.name }, to: "/loops/$name" } },
  }),
  loader: ({ context, location, params }) =>
    location.pathname.split("/").filter(Boolean).length === 2
      ? preloadLoopDetailRoute(context.queryClient, params.name)
      : Promise.resolve(),
  component: createOsRouteSync("loops"),
});
