import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/providers")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Providers" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsProvidersRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
