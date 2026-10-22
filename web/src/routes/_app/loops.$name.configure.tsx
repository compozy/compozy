import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name/configure")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Configure" } },
  }),
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, deps, params }) =>
    (await import("./-loops-preload")).preloadLoopConfigureRoute(
      context.queryClient,
      params.name,
      deps.workspace
    ),
  component: createOsRouteSync("loops"),
});
