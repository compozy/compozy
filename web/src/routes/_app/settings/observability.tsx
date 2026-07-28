import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsObservabilityRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/observability")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Observability" } },
  }),
  loader: ({ context }) => preloadSettingsObservabilityRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
