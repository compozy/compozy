import { createFileRoute } from "@tanstack/react-router";

import { automationListLoopFilter, validateJobsSearch } from "@/systems/automation";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

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
  loader: async ({ context, deps }) =>
    (await import("./-automation-preload")).preloadAutomationJobsRoute(context.queryClient, deps),
  component: createOsRouteSync("jobs"),
});
