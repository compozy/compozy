import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/jobs/$jobId")({
  beforeLoad: ({ params }): { topbar: TopbarRouteContext } => ({
    // Parent `/jobs` crumb already supplies the Jobs link — do not re-add parentCrumb.
    topbar: { crumb: { label: params.jobId } },
  }),
  loader: async ({ context, params }) =>
    (await import("./-automation-preload")).preloadAutomationJobDetailRoute(
      context.queryClient,
      params.jobId
    ),
  component: createOsRouteSync("jobs"),
});
