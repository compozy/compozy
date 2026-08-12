import { createFileRoute } from "@tanstack/react-router";

import type { TopbarRouteContext } from "@/types/topbar";
import { createOsRouteSync } from "@/systems/os";

export const Route = createFileRoute("/_app/settings/hooks")({
  beforeLoad: (): { topbar: TopbarRouteContext } => ({ topbar: { crumb: { label: "Hooks" } } }),
  loader: async ({ context }) =>
    (await import("../-settings-preload")).preloadSettingsHooksRoute(context.queryClient),
  component: createOsRouteSync("settings"),
});
