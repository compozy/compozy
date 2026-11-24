import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/computers")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Computers" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsComputersRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
