import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/extensions")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Extensions" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsExtensionsRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
