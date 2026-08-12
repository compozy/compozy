import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/roles")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Roles" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsRolesRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
