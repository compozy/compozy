import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsProvidersRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/providers")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Providers" } },
  }),
  loader: ({ context }) => preloadSettingsProvidersRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
