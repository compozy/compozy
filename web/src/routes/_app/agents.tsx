import { Outlet, createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/agents")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Agents", to: "/agents" } },
  }),
  component: AgentsRouteLayout,
});

/**
 * Structural layout for every agent route (ADR-008): it owns the shared breadcrumb and the outlet,
 * nothing else. Fleet search validation and preloading belong to the index leaf rather than running
 * for every descendant. A route-sync component here would also replace the outlet and leave the
 * whole subtree below it unmounted.
 */
function AgentsRouteLayout() {
  return <Outlet />;
}
