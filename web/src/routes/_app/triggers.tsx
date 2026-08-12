import { createFileRoute } from "@tanstack/react-router";

import { automationListLoopFilter, validateTriggersSearch } from "@/systems/automation";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/triggers")({
  validateSearch: validateTriggersSearch,
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Triggers", to: "/triggers" } },
  }),
  loaderDeps: ({ search }) => ({
    enabled: search.enabled,
    event: search.event,
    loop: automationListLoopFilter(search),
    q: search.q,
    scope: search.scope,
    source: search.source,
  }),
  loader: async ({ context, deps }) =>
    (await import("./-automation-preload")).preloadAutomationTriggersRoute(
      context.queryClient,
      deps
    ),
  component: createOsRouteSync("triggers"),
});
