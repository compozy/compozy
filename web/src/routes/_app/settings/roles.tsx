import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsRolesRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/roles")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Roles" } },
  }),
  loader: ({ context }) => preloadSettingsRolesRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
