import { createFileRoute } from "@tanstack/react-router";

import { validateLoopRunsSearch } from "@/systems/os/apps/loops/use-loop-runs-route";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadLoopRunsRoute } from "./-loops-preload";

export const Route = createFileRoute("/_app/loop-runs")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Runs", to: "/loop-runs" } },
  }),
  loaderDeps: ({ search }) => ({
    origin: search.origin,
    origin_session: search.origin_session,
  }),
  loader: ({ context, deps, location }) =>
    location.pathname.split("/").filter(Boolean).length === 1
      ? preloadLoopRunsRoute(context.queryClient, deps)
      : Promise.resolve(),
  validateSearch: validateLoopRunsSearch,
  component: createOsRouteSync("loops"),
});
