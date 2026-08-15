import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.name, params: { name: params.name }, to: "/loops/$name" } },
  }),
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, deps, location, params }) =>
    location.pathname.split("/").filter(Boolean).length === 2
      ? (await import("./-loops-preload")).preloadLoopDetailRoute(
          context.queryClient,
          params.name,
          deps.workspace
        )
      : undefined,
  component: createOsRouteSync("loops"),
});
