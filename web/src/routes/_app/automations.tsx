import { createFileRoute, redirect } from "@tanstack/react-router";

import {
  automationListLoopFilter,
  canonicalAutomationsSearch,
  validateAutomationsSearch,
} from "@/systems/automation";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/automations")({
  validateSearch: validateAutomationsSearch,
  beforeLoad: ({ location }): { topbar: TopbarRouteContext } => {
    // The listing URL normalizes in place: `?start=bogus&q=` → `/automations`.
    if (location.pathname === "/automations") {
      const canonical = canonicalAutomationsSearch(location.search as Record<string, unknown>);
      if (canonical) throw redirect({ to: "/automations", search: canonical, replace: true });
    }
    return { topbar: { crumb: { label: "Automations", to: "/automations" } } };
  },
  loaderDeps: ({ search }) => ({
    enabled: search.enabled,
    loop: automationListLoopFilter(search),
    q: search.q,
    scope: search.scope,
    source: search.source,
    start: search.start,
    target: search.target,
  }),
  loader: async ({ context, deps, location }) =>
    location.pathname.split("/").filter(Boolean).length === 1
      ? (await import("./-automation-preload")).preloadAutomationsRoute(context.queryClient, deps)
      : undefined,
  component: createOsRouteSync("automations"),
});
