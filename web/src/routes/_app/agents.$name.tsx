import { Outlet, createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/agents/$name")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: params.name, params: { name: params.name }, to: "/agents/$name" } },
  }),
  component: AgentRouteLayout,
});

/**
 * Structural layout for the agent subtree (ADR-008): it owns the shared breadcrumb and the outlet,
 * nothing else. Agent-detail search, preload, and OS synchronization belong to the index leaf, and
 * descendants own their own search — an ancestor validator here would normalize child-owned keys
 * such as the session confirmation state away before their leaf could read them.
 */
function AgentRouteLayout() {
  return <Outlet />;
}
