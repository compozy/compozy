import { createFileRoute } from "@tanstack/react-router";

import { validateLoopRunsSearch } from "@/systems/loops";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/loop-runs")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Runs", to: "/loop-runs" } },
  }),
  loaderDeps: ({ search }) => ({
    origin: search.origin,
    origin_session: search.origin_session,
    workspace: search.workspace,
  }),
  loader: async ({ context, deps, location }) =>
    location.pathname.split("/").filter(Boolean).length === 1
      ? (await import("./-loops-preload")).preloadLoopRunsRoute(
          context.queryClient,
          { origin: deps.origin, origin_session: deps.origin_session },
          deps.workspace
        )
      : undefined,
  validateSearch: validateLoopRunsSearch,
  component: createOsRouteSync("loops"),
});
