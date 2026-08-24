import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";

export const Route = createFileRoute("/_app/settings/defaults")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Defaults" } },
  }),
  loader: async ({ context }) => {
    const { preloadSettingsDefaultsRoute } = await import("../-settings-preload");
    await preloadSettingsDefaultsRoute(context.queryClient);
  },
  component: createOsRouteSync("settings"),
});
