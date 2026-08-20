import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/observability")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Diagnostics" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsObservabilityRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
