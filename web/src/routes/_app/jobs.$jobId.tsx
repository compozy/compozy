import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadAutomationJobDetailRoute } from "./-automation-preload";

export const Route = createFileRoute("/_app/jobs/$jobId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    // Parent `/jobs` crumb already supplies the Jobs link — do not re-add parentCrumb.
    topbar: { crumb: { label: params.jobId } },
  }),
  loader: ({ context, params }) =>
    preloadAutomationJobDetailRoute(context.queryClient, params.jobId),
  component: createOsRouteSync("jobs"),
});
