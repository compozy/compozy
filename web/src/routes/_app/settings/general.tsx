import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsGeneralRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/general")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "General" } },
  }),
  loader: ({ context }) => preloadSettingsGeneralRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
