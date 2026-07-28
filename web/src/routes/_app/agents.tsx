import { createFileRoute } from "@tanstack/react-router";

import { validateAgentsFleetSearch } from "@/systems/agent";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAgentsRoute } from "./-agents-preload";

export const Route = createFileRoute("/_app/agents")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Agents", to: "/agents" } },
  }),
  validateSearch: validateAgentsFleetSearch,
  loaderDeps: ({ search }) => ({
    q: search.q,
    category: search.category,
    status: search.status,
    limit: 50,
  }),
  loader: ({ context, deps, location }) =>
    location.pathname.split("/").filter(Boolean).length === 1
      ? preloadAgentsRoute(context.queryClient, deps)
      : Promise.resolve(),
  component: createOsRouteSync("agents"),
});
