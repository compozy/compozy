import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loops/$name/run")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Run" } },
  }),
  loaderDeps: ({ search }) => ({ workspace: search.workspace }),
  loader: async ({ context, deps, params }) =>
    (await import("./-loops-preload")).preloadLoopRunFormRoute(
      context.queryClient,
      params.name,
      deps.workspace
    ),
  component: createOsRouteSync("loops"),
});
