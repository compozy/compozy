import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";
import { validateAutomationDetailSearch } from "@/systems/automation";

export const Route = createFileRoute("/_app/automations/jobs/$jobId")({
  validateSearch: validateAutomationDetailSearch,
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    // Parent `/automations` crumb already supplies the Automations link.
    topbar: { crumb: { label: params.jobId } },
  }),
  loaderDeps: ({ search }) => ({ fromBell: search.from_bell }),
  loader: async ({ context, params, deps }) =>
    (await import("./-automation-preload")).preloadAutomationJobDetailRoute(
      context.queryClient,
      params.jobId,
      deps.fromBell
    ),
  component: createOsRouteSync("automations"),
});
