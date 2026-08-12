import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/general")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "General" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsGeneralRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
