import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsExtensionsRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/extensions")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Extensions" } },
  }),
  loader: ({ context }) => preloadSettingsExtensionsRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
