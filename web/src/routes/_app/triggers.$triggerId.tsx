import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAutomationTriggerDetailRoute } from "./-automation-preload";

export const Route = createFileRoute("/_app/triggers/$triggerId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    // Parent `/triggers` crumb already supplies the Triggers link — do not re-add parentCrumb.
    topbar: { crumb: { label: params.triggerId } },
  }),
  loader: ({ context, params }) =>
    preloadAutomationTriggerDetailRoute(context.queryClient, params.triggerId),
  component: createOsRouteSync("triggers"),
});
