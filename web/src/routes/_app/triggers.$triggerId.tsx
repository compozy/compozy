import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/triggers/$triggerId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    // Parent `/triggers` crumb already supplies the Triggers link — do not re-add parentCrumb.
    topbar: { crumb: { label: params.triggerId } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-automation-preload")).preloadAutomationTriggerDetailRoute(
      context.queryClient,
      params.triggerId
    ),
  component: createOsRouteSync("triggers"),
});
