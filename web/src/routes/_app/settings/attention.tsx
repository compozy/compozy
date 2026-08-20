import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/attention")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Notifications" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsAttentionRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
