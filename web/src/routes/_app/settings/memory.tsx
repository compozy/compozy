import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/memory")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Memory" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsMemoryRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
