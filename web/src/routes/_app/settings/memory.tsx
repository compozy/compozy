import { createFileRoute } from "@tanstack/react-router";

import { createOsRouteSync } from "@/systems/os";
import type { TopbarRouteContext } from "@/types/topbar";
import { preloadSettingsMemoryRoute } from "../-settings-preload";

export const Route = createFileRoute("/_app/settings/memory")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Memory" } },
  }),
  loader: ({ context }) => preloadSettingsMemoryRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
