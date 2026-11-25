import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/account")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({
    topbar: { crumb: { label: "Account" } },
  }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsAccountRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
