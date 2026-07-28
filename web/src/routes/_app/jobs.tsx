import { createFileRoute } from "@tanstack/react-router";

import {
  automationListLoopFilter,
  validateJobsSearch,
} from "@/systems/os/apps/automation/use-automation-page";
import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAutomationJobsRoute } from "./-automation-preload";

export const Route = createFileRoute("/_app/jobs")({
  validateSearch: validateJobsSearch,
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Jobs", to: "/jobs" } },
  }),
  loaderDeps: ({ search }) => ({
    enabled: search.enabled,
    loop: automationListLoopFilter(search),
    q: search.q,
    scope: search.scope,
    source: search.source,
  }),
  loader: ({ context, deps }) => preloadAutomationJobsRoute(context.queryClient, deps),
  component: createOsRouteSync("jobs"),
});
