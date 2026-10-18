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
  loader: async ({ context, params }) =>
    (await import("./-automation-preload")).preloadAutomationJobDetailRoute(
      context.queryClient,
      params.jobId
    ),
  component: createOsRouteSync("automations"),
});
