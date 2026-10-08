import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/automations/triggers/$triggerId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    // Parent `/automations` crumb already supplies the Automations link.
    topbar: { crumb: { label: params.triggerId } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-automation-preload")).preloadAutomationTriggerDetailRoute(
      context.queryClient,
      params.triggerId
    ),
  component: createOsRouteSync("automations"),
});
