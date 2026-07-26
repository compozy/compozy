import { createFileRoute } from "@tanstack/react-router";

import {
  automationListLoopFilter,
  validateTriggersSearch,
} from "@/systems/os/apps/automation/use-automation-page";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAutomationTriggersRoute } from "./-automation-preload";

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
  loader: ({ context, deps }) => preloadAutomationTriggersRoute(context.queryClient, deps),
  component: createOsRouteSync("triggers"),
});
