import { createFileRoute } from "@tanstack/react-router";

import { automationListLoopFilter, validateAutomationsSearch } from "@/systems/automation";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/automations")({
  validateSearch: validateAutomationsSearch,
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Automations", to: "/automations" } },
  }),
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
